import { screen, waitFor, within } from "@testing-library/react";
import { http, HttpResponse } from "msw";
import { describe, expect, it } from "vitest";
import { fmtDateTime } from "@/shared/lib/format";
import { api, apiError } from "@/shared/test/msw";
import { STAGE_NOT_FOUND } from "../model/messages";
import type { ClassMatrix, MemberStage } from "../model/schemas";
import { fixtures } from "../test/golden";
import { TAB_PATH, polyfillRadix, renderSummaryTab } from "../test/render-summary";

polyfillRadix();

const matrixUrl = api("/classes/:classId/summary");
const drawerUrl = api("/classes/:classId/members/:memberId/stages/:stageVersionId/summary");
const reviewUrl = api("/classes/:classId/members/:memberId/stages/:stageVersionId/review");
const [db, ds] = fixtures.matrix.stages.map((s) => s.stageVersionId);

/** A matrix cell; `hidden` reaches it behind the open (modal) drawer. */
const cellOf = (name: string, stage: string, hidden = false) =>
  screen.getByRole("button", { name: new RegExp(`^${name}, ${stage}: `), hidden });
const rowNames = () =>
  within(screen.getByRole("table", { name: "Tổng kết theo chặng lớp basic01" }))
    .queryAllByRole("rowheader")
    .map((th) => th.querySelector("span > span")?.textContent);
const searchOf = (router: { state: { location: { search: string } } }) => new URLSearchParams(router.state.location.search);
const matrixOf = (m: ClassMatrix) => http.get(matrixUrl, () => HttpResponse.json(m));

/** Trần Thị Bích's Data structure stage, not reviewed yet. */
const bichStage: MemberStage = {
  ...fixtures.memberStageNoReview,
  member: { ...fixtures.memberStageNoReview.member, name: "Trần Thị Bích", email: "bich.tran@gmail.com" },
};

describe("SummaryTab matrix", () => {
  it("shows every member against every stage with the course column, the count and the legend", async () => {
    renderSummaryTab();

    const table = await screen.findByRole("table", { name: "Tổng kết theo chặng lớp basic01" });
    expect(screen.getByText("7/7 học viên · Bấm một ô để xem tổng kết chặng và viết nhận xét")).toBeInTheDocument();
    expect(within(table).getAllByRole("columnheader").map((th) => th.textContent)).toEqual([
      "Học viên",
      "Database",
      "Data structure",
      "Golang basic",
      "ReactJS basic",
      "HTML CSS JS",
      "Toàn khóa",
    ]);
    expect(rowNames()).toHaveLength(7);
    const legend = screen.getByRole("list", { name: "Chú giải màu ô" });
    expect(within(legend).getAllByRole("listitem").map((li) => li.textContent)).toEqual([
      "Hoàn thành",
      "Đang học",
      "Cần làm lại",
      "Quá hạn",
    ]);
    // The matrix scrolls sideways inside its own focusable region.
    expect(screen.getByRole("region", { name: "Ma trận tổng kết lớp basic01" })).toHaveAttribute("tabindex", "0");
  });

  it("says each cell's state in words, not only in colour", async () => {
    renderSummaryTab();
    await screen.findByRole("table", { name: "Tổng kết theo chặng lớp basic01" });

    const an = cellOf("Nguyễn Hoàng An", "Data structure");
    expect(an).toHaveAccessibleName("Nguyễn Hoàng An, Data structure: Hoàn thành, đã nhận xét. Mở tổng kết chặng");
    expect(an).toHaveTextContent("Hoàn thành");
    const ha = cellOf("Hoàng Thu Hà", "Data structure");
    expect(ha).toHaveAccessibleName(
      "Hoàng Thu Hà, Data structure: Chưa hoàn thành, 2/6 bắt buộc, còn 4 mục, có bài quá hạn, chưa nhận xét. Mở tổng kết chặng",
    );
    expect(ha).toHaveTextContent("2/6 bắt buộc");
    expect(ha).toHaveTextContent("Còn 4 mục");
    expect(ha).toHaveTextContent("Quá hạn");
    const row = screen.getByRole("rowheader", { name: /Nguyễn Hoàng An/ }).closest("tr");
    expect(within(row as HTMLElement).getByRole("img", { name: "64% hoàn thành" })).toBeInTheDocument();
  });

  it("filters from the URL and keeps the tab", async () => {
    renderSummaryTab(`${TAB_PATH}&overdue=1`);

    expect(await screen.findByText(/^3\/7 học viên/)).toBeInTheDocument();
    expect(rowNames()).toEqual(["Hoàng Thu Hà", "Phạm Minh Dũng", "Vũ Đức Khang"]);
    expect(screen.getByRole("checkbox", { name: "Có bài quá hạn nộp" })).toBeChecked();
    expect(screen.getByRole("link", { name: "Xóa bộ lọc" })).toHaveAttribute("href", TAB_PATH);
  });

  it("applies the stage filters only on Lọc", async () => {
    const { user, router } = renderSummaryTab();
    await screen.findByText(/^7\/7 học viên/);

    await user.click(screen.getByRole("combobox", { name: "Chưa hoàn thành chặng" }));
    await user.click(await screen.findByRole("option", { name: "Database" }));
    await user.click(screen.getByRole("combobox", { name: "Chưa có nhận xét chặng" }));
    await user.click(await screen.findByRole("option", { name: "Data structure" }));
    expect(screen.getByText(/^7\/7 học viên/)).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "Lọc" }));

    await waitFor(() => {
      expect(searchOf(router).toString()).toBe(`tab=summary&stage=${db}&noReview=${ds}`);
    });
    expect(screen.getByText(/^4\/7 học viên/)).toBeInTheDocument();
    expect(rowNames()).toEqual(["Hoàng Thu Hà", "Phạm Minh Dũng", "Vũ Đức Khang", "Đặng Hải Phong"]);
    expect(screen.getByRole("combobox", { name: "Chưa hoàn thành chặng" })).toHaveTextContent("Database");
  });

  it("finds the members with a homework to redo", async () => {
    const { user, router } = renderSummaryTab(TAB_PATH, { overrides: [matrixOf(fixtures.matrixWithRework)] });
    await screen.findByText(/^7\/7 học viên/);

    await user.click(screen.getByRole("checkbox", { name: "Có bài cần làm lại" }));
    await user.click(screen.getByRole("button", { name: "Lọc" }));

    await waitFor(() => { expect(searchOf(router).get("rework")).toBe("1"); });
    expect(screen.getByText(/^1\/7 học viên/)).toBeInTheDocument();
    expect(cellOf("Trần Thị Bích", "Data structure")).toHaveAccessibleName(
      "Trần Thị Bích, Data structure: Chưa hoàn thành, 4/6 bắt buộc, còn 2 mục, có bài cần làm lại, chưa nhận xét. Mở tổng kết chặng",
    );
    expect(cellOf("Trần Thị Bích", "Data structure")).toHaveTextContent("Làm lại");
  });

  it("ignores a stage of another course in the URL", async () => {
    renderSummaryTab(`${TAB_PATH}&stage=0199ffff-0000-7000-8000-999999999999`);

    expect(await screen.findByText(/^7\/7 học viên/)).toBeInTheDocument();
    expect(screen.queryByRole("link", { name: "Xóa bộ lọc" })).toBeNull();
  });

  it("explains an empty matrix with and without a filter", async () => {
    const { unmount } = renderSummaryTab(`${TAB_PATH}&rework=1`);

    expect(await screen.findByRole("heading", { name: "Không có học viên khớp bộ lọc" })).toBeInTheDocument();
    expect(screen.getByText("Nới lỏng điều kiện hoặc xóa bộ lọc.")).toBeInTheDocument();
    expect(screen.getByText(/^0\/7 học viên/)).toBeInTheDocument();
    expect(screen.getAllByRole("link", { name: "Xóa bộ lọc" }).every((l) => l.getAttribute("href") === TAB_PATH)).toBe(true);
    unmount();

    renderSummaryTab(TAB_PATH, { overrides: [matrixOf({ ...fixtures.matrix, rows: [] })] });
    expect(await screen.findByRole("heading", { name: "Chưa có học viên" })).toBeInTheDocument();
    expect(screen.getByText("Mời học viên vào lớp để xem tổng kết.")).toBeInTheDocument();
  });

  it("shows the API's refusal with a retry", async () => {
    let fail = true;
    const { user } = renderSummaryTab(TAB_PATH, {
      overrides: [
        http.get(matrixUrl, () =>
          fail
            ? apiError(403, "FORBIDDEN", "Bạn chỉ xem được lớp mình phụ trách.")
            : HttpResponse.json(fixtures.matrix),
        ),
      ],
    });

    expect(await screen.findByRole("alert")).toHaveTextContent("Bạn chỉ xem được lớp mình phụ trách.");
    fail = false;
    await user.click(screen.getByRole("button", { name: "Thử lại" }));
    expect(await screen.findByText(/^7\/7 học viên/)).toBeInTheDocument();
  });
});

describe("SummaryTab stage drawer", () => {
  it("opens a cell with the keyboard and gives focus back to it on close", async () => {
    const { user } = renderSummaryTab();
    await screen.findByText(/^7\/7 học viên/);

    cellOf("Nguyễn Hoàng An", "Data structure").focus();
    await user.keyboard("{Enter}");

    const drawer = await screen.findByRole("dialog", { name: "Tổng kết chặng" });
    expect(await within(drawer).findByRole("heading", { name: "Nguyễn Hoàng An · Data structure" })).toBeInTheDocument();
    expect(within(drawer).getByText("DS · v1 · Đã hoàn thành chặng")).toBeInTheDocument();
    expect(
      within(drawer).getByText(
        "6/6 học liệu bắt buộc · video và bài đọc do học viên tự xác nhận; quiz và bài tập tính theo kết quả",
      ),
    ).toBeInTheDocument();
    const body = within(drawer).getByRole("region", { name: "Học liệu và nhận xét của chặng" });
    expect(within(body).getByRole("heading", { level: 3, name: "Mảng, Stack và Queue" })).toBeInTheDocument();
    expect(within(body).getByText("Bài tập: Cài đặt Stack và Queue")).toBeInTheDocument();
    expect(within(body).getByText(/^Lần nộp #2 · .* · nộp muộn · 94\/100 điểm$/)).toBeInTheDocument();
    expect(within(body).getByText("Tốt nhất 100% (5/5) · 1/2 lượt · Đã xác nhận")).toBeInTheDocument();
    // Staff read the student's lessons here; they open them from the course editor, not from a student's link.
    expect(within(body).queryByRole("link", { name: "Bài tập: Cài đặt Stack và Queue" })).toBeNull();

    const form = within(drawer).getByRole("form", { name: "Nhận xét tổng kết chặng" });
    const { review } = fixtures.memberStage.summary;
    expect(within(form).getByText(`Cập nhật ${fmtDateTime(review?.updatedAt ?? "")} bởi Lê Thu Hương`)).toBeInTheDocument();
    expect(within(form).getByRole("textbox", { name: "Nhận xét" })).toHaveValue(review?.body);
    expect(within(form).getByRole("textbox", { name: "Nhận xét" })).toHaveAccessibleDescription(
      "Học viên thấy nhận xét này trong trang tổng kết của mình.",
    );
    expect(within(form).getByRole("button", { name: "Lưu thay đổi" })).toBeInTheDocument();

    await user.keyboard("{Escape}");
    await waitFor(() => { expect(screen.queryByRole("dialog")).toBeNull(); });
    await waitFor(() => { expect(cellOf("Nguyễn Hoàng An", "Data structure")).toHaveFocus(); });
  });

  it("says how many required lessons an incomplete stage still misses", async () => {
    const incomplete: MemberStage = {
      ...fixtures.memberStage,
      summary: {
        ...fixtures.memberStage.summary,
        complete: false,
        done: 4,
        missing: [
          { lessonId: "a", title: "Hash table", unitTitle: "Hash table" },
          { lessonId: "b", title: "Kiểm tra cuối chặng Data structure", unitTitle: "Kiểm tra cuối chặng" },
        ],
      },
    };
    const { user } = renderSummaryTab(TAB_PATH, { memberStage: incomplete });
    await screen.findByText(/^7\/7 học viên/);

    await user.click(cellOf("Trần Thị Bích", "Data structure"));

    const drawer = await screen.findByRole("dialog", { name: "Tổng kết chặng" });
    expect(await within(drawer).findByText("DS · v1 · Còn 2 học liệu bắt buộc")).toBeInTheDocument();
  });

  it("writes a review, flags the cell as reviewed, then edits it", async () => {
    const { user } = renderSummaryTab(TAB_PATH, { memberStage: bichStage });
    await screen.findByText(/^7\/7 học viên/);
    expect(cellOf("Trần Thị Bích", "Data structure")).toHaveAccessibleName(/chưa nhận xét\. Mở tổng kết chặng$/);

    await user.click(cellOf("Trần Thị Bích", "Data structure"));
    const drawer = await screen.findByRole("dialog", { name: "Tổng kết chặng" });
    const form = await within(drawer).findByRole("form", { name: "Nhận xét tổng kết chặng" });
    expect(within(form).queryByText(/^Cập nhật /)).toBeNull();
    await user.type(within(form).getByRole("textbox", { name: "Nhận xét" }), "  Bích nắm chắc stack.  ");
    await user.click(within(form).getByRole("button", { name: "Lưu nhận xét" }));

    expect(await screen.findByText("Đã lưu nhận xét.")).toBeInTheDocument();
    expect(await within(form).findByText(/^Cập nhật .* bởi Lê Thu Hương$/)).toBeInTheDocument();
    expect(within(form).getByRole("textbox", { name: "Nhận xét" })).toHaveValue("Bích nắm chắc stack.");
    expect(within(form).getByRole("button", { name: "Lưu thay đổi" })).toBeInTheDocument();
    // The matrix behind the drawer reloads, so the "no review" filter no longer finds this cell.
    await waitFor(() => {
      expect(cellOf("Trần Thị Bích", "Data structure", true)).toHaveAccessibleName(/đã nhận xét\. Mở tổng kết chặng$/);
    });

    await user.type(within(form).getByRole("textbox", { name: "Nhận xét" }), " Cần luyện thêm hash.");
    await user.click(within(form).getByRole("button", { name: "Lưu thay đổi" }));

    expect(await within(form).findByText(/^Cập nhật .* bởi Lê Thu Hương · đã sửa 1 lần$/)).toBeInTheDocument();
    expect(within(form).getByRole("textbox", { name: "Nhận xét" })).toHaveValue("Bích nắm chắc stack. Cần luyện thêm hash.");
  });

  it("checks the review before sending it and shows the API's validation on the field", async () => {
    let calls = 0;
    const { user } = renderSummaryTab(TAB_PATH, {
      memberStage: bichStage,
      overrides: [
        http.put(reviewUrl, () => {
          calls += 1;
          return apiError(422, "VALIDATION_FAILED", "Nhận xét tối đa 5000 ký tự.");
        }),
      ],
    });
    await screen.findByText(/^7\/7 học viên/);
    await user.click(cellOf("Trần Thị Bích", "Data structure"));
    const form = await screen.findByRole("form", { name: "Nhận xét tổng kết chặng" });
    const field = within(form).getByRole("textbox", { name: "Nhận xét" });

    await user.type(field, "   ");
    await user.click(within(form).getByRole("button", { name: "Lưu nhận xét" }));
    expect(field).toHaveAccessibleDescription(/Nhập nhận xét tổng kết chặng\.$/);
    expect(field).toHaveAttribute("aria-invalid", "true");
    expect(calls).toBe(0);

    await user.type(field, "Ổn.");
    await user.click(within(form).getByRole("button", { name: "Lưu nhận xét" }));
    await waitFor(() => { expect(field).toHaveAccessibleDescription(/Nhận xét tối đa 5000 ký tự\.$/); });
    expect(calls).toBe(1);
    expect(within(form).queryByRole("alert")).toBeNull();
  });

  it("keeps the text and reloads the drawer when someone else saved in between", async () => {
    let reads = 0;
    const { user } = renderSummaryTab(TAB_PATH, {
      overrides: [
        http.get(drawerUrl, () => {
          reads += 1;
          return HttpResponse.json(reads === 1 ? bichStage : fixtures.memberStage);
        }),
        http.put(reviewUrl, () =>
          apiError(409, "CONFLICT", "Nhận xét vừa được người khác lưu. Tải lại rồi thử lại."),
        ),
      ],
    });
    await screen.findByText(/^7\/7 học viên/);
    await user.click(cellOf("Trần Thị Bích", "Data structure"));
    const form = await screen.findByRole("form", { name: "Nhận xét tổng kết chặng" });

    await user.type(within(form).getByRole("textbox", { name: "Nhận xét" }), "Bản của tôi");
    await user.click(within(form).getByRole("button", { name: "Lưu nhận xét" }));

    expect(await within(form).findByRole("alert")).toHaveTextContent(
      "Nhận xét vừa được người khác lưu. Tải lại rồi thử lại.",
    );
    // The other teacher's review shows above the field; what this teacher typed is kept.
    expect(await within(form).findByText(/^Cập nhật .* bởi Lê Thu Hương$/)).toBeInTheDocument();
    expect(within(form).getByRole("textbox", { name: "Nhận xét" })).toHaveValue("Bản của tôi");
    expect(reads).toBe(2);
  });

  it("closes with a toast when the member or stage is not part of the class", async () => {
    const { user } = renderSummaryTab(TAB_PATH, {
      overrides: [http.get(drawerUrl, () => apiError(404, "NOT_FOUND", "Chặng không thuộc khóa học của lớp này."))],
    });
    await screen.findByText(/^7\/7 học viên/);

    cellOf("Hoàng Thu Hà", "Golang basic").focus();
    await user.keyboard("{Enter}");

    expect(await screen.findByText(STAGE_NOT_FOUND)).toBeInTheDocument();
    await waitFor(() => { expect(screen.queryByRole("dialog")).toBeNull(); });
    await waitFor(() => { expect(cellOf("Hoàng Thu Hà", "Golang basic")).toHaveFocus(); });
  });

  it("shows a refusal in the drawer with a retry", async () => {
    let fail = true;
    const { user } = renderSummaryTab(TAB_PATH, {
      overrides: [
        http.get(drawerUrl, () =>
          fail
            ? apiError(403, "FORBIDDEN", "Bạn chỉ nhận xét học viên của lớp mình phụ trách.")
            : HttpResponse.json(fixtures.memberStage),
        ),
      ],
    });
    await screen.findByText(/^7\/7 học viên/);

    await user.click(cellOf("Nguyễn Hoàng An", "Data structure"));
    const drawer = await screen.findByRole("dialog", { name: "Tổng kết chặng" });
    expect(await within(drawer).findByRole("alert")).toHaveTextContent(
      "Bạn chỉ nhận xét học viên của lớp mình phụ trách.",
    );

    fail = false;
    await user.click(within(drawer).getByRole("button", { name: "Thử lại" }));
    expect(await within(drawer).findByRole("heading", { name: "Nguyễn Hoàng An · Data structure" })).toBeInTheDocument();
  });
});
