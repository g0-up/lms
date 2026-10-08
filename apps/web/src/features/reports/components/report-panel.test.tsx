import { screen, waitFor, within } from "@testing-library/react";
import { http, HttpResponse } from "msw";
import { describe, expect, it } from "vitest";
import { api, apiError, server, setSession, users } from "@/shared/test/msw";
import { renderRoutes } from "@/shared/test/render";
import { reportsHandlers } from "../api/msw-handlers";
import type { ClassReport } from "../model/schemas";
import { fixtures, ids } from "../test/golden";
import { gate, polyfillRadix } from "../test/render-reports";
import { ReportPanel } from "./report-panel";

polyfillRadix();

const BASE = "/panel?tab=report";
const reportUrl = api("/classes/:classId/report");

/** Records the query string of every report request and answers `respond` (the golden report by default). */
function recordReports(respond: (params: URLSearchParams) => ClassReport | Response = () => fixtures.classReport) {
  const seen: URLSearchParams[] = [];
  const handler = http.get(reportUrl, ({ request }) => {
    const params = new URL(request.url).searchParams;
    seen.push(params);
    const answer = respond(params);
    return answer instanceof Response ? answer : HttpResponse.json(answer);
  });
  return { seen, handler, last: () => seen.at(-1) };
}

function renderPanel(path = BASE, overrides: Parameters<typeof server.use> = []) {
  setSession(users.admin);
  server.use(...reportsHandlers);
  // Later `use` calls win: overrides go last.
  if (overrides.length > 0) server.use(...overrides);
  return renderRoutes(
    [{ path: "/panel", element: <ReportPanel classId={ids.reportClass} basePath={BASE} /> }],
    { initialEntries: [path], withToaster: true },
  );
}

const rowOf = (name: string) => screen.getByRole("button", { name: `Xem chi tiết ${name}` });
const searchOf = (router: ReturnType<typeof renderPanel>["router"]) =>
  new URLSearchParams(router.state.location.search);

describe("ReportPanel", () => {
  it("shows one row per active member, the stage columns and the self-reported notice", async () => {
    renderPanel();

    expect(await screen.findByText("7/7 học viên · Bấm vào một dòng để xem từng học liệu")).toBeInTheDocument();
    expect(screen.getByText("Tiến độ do học viên tự xác nhận")).toBeInTheDocument();
    expect(screen.getByRole("columnheader", { name: "DB" })).toHaveAttribute("title", "Database v1");
    expect(screen.getByRole("columnheader", { name: "WEB" })).toHaveAttribute("title", "HTML CSS JS v1");

    const cuong = rowOf("Lê Văn Cường");
    expect(within(cuong).getByRole("img", { name: "93% hoàn thành" })).toBeInTheDocument();
    expect(within(cuong).getAllByText("100%")[0]).toHaveClass("text-ok");
    expect(within(cuong).getByText("67%")).not.toHaveClass("text-ok");
    expect(within(cuong).getByText("Đã gửi")).toBeInTheDocument();

    // Never active: no last activity, the invited dot instead.
    expect(within(rowOf("Phạm Minh Dũng")).getByText("Chưa đăng nhập")).toBeInTheDocument();
    expect(screen.queryByText("Đã rời lớp")).toBeNull();
  });

  it("keeps the filter bar usable while the report loads", async () => {
    const hold = gate();
    renderPanel(BASE, [
      http.get(reportUrl, async () => {
        await hold.wait();
        return HttpResponse.json(fixtures.classReport);
      }),
    ]);

    expect(screen.getByText("Đang tải báo cáo").parentElement).toHaveAttribute("aria-busy", "true");
    expect(screen.getByRole("button", { name: "Lọc" })).toBeEnabled();
    expect(screen.getByRole("checkbox", { name: "Chưa đăng nhập" })).toBeEnabled();

    hold.open();
    expect(await screen.findByText(/^7\/7 học viên/)).toBeInTheDocument();
    expect(screen.queryByText("Đang tải báo cáo")).toBeNull();
  });

  it("uses the compact 36px filter controls on desktop and 44px on touch screens", async () => {
    renderPanel();
    await screen.findByText(/^7\/7 học viên/);

    for (const control of [
      screen.getByLabelText("Không hoạt động quá (ngày)"),
      screen.getByLabelText("% toàn khóa dưới"),
      screen.getByRole("combobox", { name: "Sắp xếp" }),
    ]) {
      expect(control).toHaveClass("h-9", "max-[720px]:min-h-11");
      expect(control).not.toHaveClass("h-10");
    }
  });

  it("offers a retry when the report cannot be loaded", async () => {
    let fail = true;
    const { user } = renderPanel(BASE, [
      http.get(reportUrl, () => (fail ? apiError(500, "INTERNAL", "Lỗi máy chủ") : HttpResponse.json(fixtures.classReport))),
    ]);

    const alert = await screen.findByRole("alert");
    expect(alert).toHaveTextContent("Không tải được báo cáo.");

    fail = false;
    await user.click(within(alert).getByRole("button", { name: "Thử lại" }));
    expect(await screen.findByText(/^7\/7 học viên/)).toBeInTheDocument();
  });

  it("writes the filter to the URL with short names and asks the API with its own names", async () => {
    const reports = recordReports();
    const { user, router } = renderPanel(BASE, [reports.handler]);
    await screen.findByText(/^7\/7 học viên/);

    await user.click(screen.getByRole("checkbox", { name: "Chưa đăng nhập" }));
    await user.type(screen.getByLabelText("Không hoạt động quá (ngày)"), "7");
    await user.type(screen.getByLabelText("% toàn khóa dưới"), "50");
    await user.click(screen.getByRole("button", { name: "Lọc" }));

    await waitFor(() => { expect(reports.seen).toHaveLength(2); });
    const url = searchOf(router);
    expect(url.get("tab")).toBe("report");
    expect(url.get("notlogged")).toBe("1");
    expect(url.get("inactive")).toBe("7");
    expect(url.get("below")).toBe("50");

    expect(Object.fromEntries(reports.last() ?? [])).toEqual({
      notLoggedIn: "true",
      inactiveDays: "7",
      belowPercent: "50",
    });
    expect(screen.getByRole("link", { name: "Xóa bộ lọc" })).toHaveAttribute("href", BASE);
  });

  it("reads a shared link's filter back into the fields and the request", async () => {
    const reports = recordReports();
    renderPanel(`${BASE}&below=50&inactive=14&notlogged=1&sort=pct-desc`, [reports.handler]);
    await screen.findByText(/học viên · Bấm vào một dòng/);

    expect(Object.fromEntries(reports.last() ?? [])).toEqual({
      notLoggedIn: "true",
      inactiveDays: "14",
      belowPercent: "50",
      sort: "pct-desc",
    });
    expect(screen.getByLabelText("% toàn khóa dưới")).toHaveValue(50);
    expect(screen.getByLabelText("Không hoạt động quá (ngày)")).toHaveValue(14);
    expect(screen.getByRole("checkbox", { name: "Chưa đăng nhập" })).toBeChecked();
    expect(screen.getByRole("combobox", { name: "Sắp xếp" })).toHaveTextContent("% toàn khóa cao → thấp");
    expect(screen.getByRole("columnheader", { name: /Toàn khóa/ })).toHaveAttribute("aria-sort", "descending");
  });

  it("drops malformed URL values instead of forwarding them", async () => {
    const reports = recordReports();
    renderPanel(`${BASE}&below=abc&inactive=0&sort=bogus&notlogged=yes&belowPercent=10`, [reports.handler]);
    await screen.findByText(/^7\/7 học viên/);

    expect(reports.last()?.toString()).toBe("");
    expect(screen.queryByRole("link", { name: "Xóa bộ lọc" })).toBeNull();
  });

  it("applies the sort chosen in the select", async () => {
    const reports = recordReports();
    const { user, router } = renderPanel(BASE, [reports.handler]);
    await screen.findByText(/^7\/7 học viên/);

    await user.click(screen.getByRole("combobox", { name: "Sắp xếp" }));
    await user.click(await screen.findByRole("option", { name: "Lâu không hoạt động" }));
    await user.click(screen.getByRole("button", { name: "Lọc" }));

    await waitFor(() => { expect(searchOf(router).get("sort")).toBe("activity-asc"); });
    await waitFor(() => { expect(reports.last()?.get("sort")).toBe("activity-asc"); });
    expect(screen.getByRole("columnheader", { name: /Hoạt động cuối/ })).toHaveAttribute("aria-sort", "ascending");
  });

  it("shows dropped members on request without changing the active count", async () => {
    const reports = recordReports((params) =>
      params.get("includeDropped") === "true" ? fixtures.classReportWithDropped : fixtures.classReport,
    );
    const { user, router } = renderPanel(BASE, [reports.handler]);
    await screen.findByText(/^7\/7 học viên/);
    expect(screen.queryByRole("button", { name: "Xem chi tiết Ngô Thanh Tâm" })).toBeNull();

    await user.click(screen.getByRole("checkbox", { name: "Hiện học viên đã rời lớp" }));
    await user.click(screen.getByRole("button", { name: "Lọc" }));

    const dropped = await screen.findByRole("button", { name: "Xem chi tiết Ngô Thanh Tâm" });
    expect(within(dropped).getByText("Đã rời lớp")).toBeInTheDocument();
    // A dropped row is tinted, never faded, so its text keeps the 4.5:1 contrast.
    expect(dropped).toHaveClass("bg-surface-100");
    expect(dropped).not.toHaveClass("opacity-60");
    expect(searchOf(router).get("dropped")).toBe("1");
    expect(reports.last()?.get("includeDropped")).toBe("true");
    expect(screen.getByText("7/7 học viên · Bấm vào một dòng để xem từng học liệu")).toBeInTheDocument();
    // The dropped toggle alone narrows nothing, so there is nothing to clear.
    expect(screen.queryByRole("link", { name: "Xóa bộ lọc" })).toBeNull();
  });

  it("toggles the column sort and marks the sorted header", async () => {
    const reports = recordReports();
    const { user, router } = renderPanel(BASE, [reports.handler]);
    await screen.findByText(/^7\/7 học viên/);
    const pctHead = () => screen.getByRole("columnheader", { name: /Toàn khóa/ });
    const activityHead = () => screen.getByRole("columnheader", { name: /Hoạt động cuối/ });
    expect(pctHead()).not.toHaveAttribute("aria-sort");

    await user.click(within(pctHead()).getByRole("button", { name: /Toàn khóa/ }));
    await waitFor(() => { expect(pctHead()).toHaveAttribute("aria-sort", "ascending"); });
    expect(searchOf(router).get("sort")).toBe("pct");
    await waitFor(() => { expect(reports.last()?.get("sort")).toBe("pct"); });

    await user.click(within(pctHead()).getByRole("button", { name: /Toàn khóa/ }));
    await waitFor(() => { expect(pctHead()).toHaveAttribute("aria-sort", "descending"); });
    expect(searchOf(router).get("sort")).toBe("pct-desc");

    await user.click(within(activityHead()).getByRole("button", { name: /Hoạt động cuối/ }));
    await waitFor(() => { expect(activityHead()).toHaveAttribute("aria-sort", "descending"); });
    expect(pctHead()).not.toHaveAttribute("aria-sort");
    expect(searchOf(router).get("sort")).toBe("activity");
    expect(searchOf(router).get("tab")).toBe("report");
  });

  it("opens a member's lessons with Enter and gives focus back on close", async () => {
    const { user } = renderPanel();
    await screen.findByText(/^7\/7 học viên/);

    rowOf("Nguyễn Hoàng An").focus();
    await user.keyboard("{Enter}");

    const drawer = await screen.findByRole("dialog", { name: "Chi tiết học viên" });
    expect(await within(drawer).findByRole("heading", { name: "Nguyễn Hoàng An" })).toBeInTheDocument();
    expect(within(drawer).getByText(/^an\.nguyen@gmail\.com · basic01 · hoạt động cuối/)).toBeInTheDocument();
    expect(within(drawer).getByText("10/14 học liệu bắt buộc · tiến độ do học viên tự xác nhận")).toBeInTheDocument();
    const stages = within(drawer).getByRole("region", { name: "Tiến độ theo chặng" });
    expect(stages).toHaveAttribute("tabindex", "0");
    expect(within(stages).getByRole("heading", { name: "1. Database v1" })).toBeInTheDocument();
    expect(within(stages).getByRole("heading", { name: "5. HTML CSS JS v1" })).toBeInTheDocument();
    expect(within(drawer).getAllByText("Không bắt buộc")).toHaveLength(2);
    expect(within(drawer).getAllByText("Chưa mở").length).toBeGreaterThan(0);
    expect(within(drawer).getAllByText(/^Tích \d{2}\/\d{2}\/\d{4} \d{2}:\d{2}$/).length).toBeGreaterThan(0);
    expect(within(drawer).getAllByText(/^Mở lần đầu /).length).toBeGreaterThan(0);

    await user.keyboard("{Escape}");
    await waitFor(() => { expect(screen.queryByRole("dialog")).toBeNull(); });
    await waitFor(() => { expect(rowOf("Nguyễn Hoàng An")).toHaveFocus(); });
  });

  it("opens the drawer with a click or Space as well", async () => {
    const { user } = renderPanel();
    await screen.findByText(/^7\/7 học viên/);

    await user.click(rowOf("Trần Thị Bích"));
    expect(await screen.findByRole("heading", { name: "Trần Thị Bích" })).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "Đóng" }));
    await waitFor(() => { expect(screen.queryByRole("dialog")).toBeNull(); });

    rowOf("Vũ Đức Khang").focus();
    await user.keyboard(" ");
    expect(await screen.findByRole("heading", { name: "Vũ Đức Khang" })).toBeInTheDocument();
  });

  it("closes the drawer with a toast when the member is no longer in the class", async () => {
    const { user } = renderPanel(BASE, [
      http.get(api("/classes/:classId/report/members/:memberId"), () =>
        apiError(404, "NOT_FOUND", "Không tìm thấy."),
      ),
    ]);
    await screen.findByText(/^7\/7 học viên/);

    rowOf("Hoàng Thu Hà").focus();
    await user.keyboard("{Enter}");

    expect(await screen.findByText("Không tìm thấy học viên trong lớp.")).toBeInTheDocument();
    await waitFor(() => { expect(screen.queryByRole("dialog")).toBeNull(); });
  });

  it("shows the drilldown error with a retry for other failures", async () => {
    let fail = true;
    const { user } = renderPanel(BASE, [
      http.get(api("/classes/:classId/report/members/:memberId"), () =>
        fail ? apiError(500, "INTERNAL", "Lỗi máy chủ") : HttpResponse.json(fixtures.memberReport),
      ),
    ]);
    await screen.findByText(/^7\/7 học viên/);

    await user.click(rowOf("Hoàng Thu Hà"));
    const drawer = await screen.findByRole("dialog", { name: "Chi tiết học viên" });
    expect(await within(drawer).findByRole("alert")).toHaveTextContent("Lỗi máy chủ");

    fail = false;
    await user.click(within(drawer).getByRole("button", { name: "Thử lại" }));
    expect(await within(drawer).findByRole("heading", { name: "Nguyễn Hoàng An" })).toBeInTheDocument();
  });

  it("explains an empty result with and without a filter", async () => {
    const empty = { ...fixtures.classReport, rows: [] };
    renderPanel(`${BASE}&below=10`, [recordReports(() => empty).handler]);

    expect(await screen.findByRole("heading", { name: "Không có học viên khớp bộ lọc" })).toBeInTheDocument();
    expect(screen.getByText("Nới lỏng điều kiện hoặc xóa bộ lọc.")).toBeInTheDocument();
    expect(screen.getAllByRole("link", { name: "Xóa bộ lọc" })).toHaveLength(2);
  });

  it("says there is nobody yet when the class has no members", async () => {
    renderPanel(BASE, [recordReports(() => ({ ...fixtures.classReport, rows: [] })).handler]);

    expect(await screen.findByRole("heading", { name: "Chưa có học viên" })).toBeInTheDocument();
    expect(screen.getByText("Mời học viên vào lớp để theo dõi tiến độ.")).toBeInTheDocument();
    expect(screen.queryByRole("link", { name: "Xóa bộ lọc" })).toBeNull();
  });
});
