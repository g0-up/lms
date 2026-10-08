import { screen, within } from "@testing-library/react";
import { http, HttpResponse, type RequestHandler } from "msw";
import { afterEach, beforeAll, beforeEach, describe, expect, it, vi } from "vitest";
import { api, apiError, server, setSession, users } from "@/shared/test/msw";
import { renderRoutes } from "@/shared/test/render";
import { dashboardHandlers } from "../api/msw-handlers";
import type { Dashboard } from "../model/schemas";
import { routes } from "../routes";
import { fixtures } from "../test/golden";

// The route is lazy: load it once up front so the first test's timeouts do not include the import.
beforeAll(async () => {
  await import("./dashboard-page");
});

function renderDashboard({ dashboard, overrides = [] }: { dashboard?: Dashboard; overrides?: RequestHandler[] } = {}) {
  setSession(users.admin);
  server.use(...dashboardHandlers(dashboard));
  if (overrides.length > 0) server.use(...overrides);
  return renderRoutes([{ path: "/admin", children: routes }], { initialEntries: ["/admin"] });
}

function card(name: string): HTMLElement {
  const el = screen.getByRole("heading", { name }).closest<HTMLElement>('[data-slot="card"]');
  if (!el) throw new Error(`no card for ${name}`);
  return el;
}

const kpi = (label: string) => {
  const el = screen.getByText(label).closest<HTMLElement>('[data-slot="card"]');
  if (!el) throw new Error(`no KPI ${label}`);
  return el;
};

beforeEach(() => {
  // Only the clock: the golden activity is two hours old.
  vi.useFakeTimers({ toFake: ["Date"] });
  vi.setSystemTime(new Date("2026-10-05T10:00:00Z"));
});

afterEach(() => {
  vi.useRealTimers();
});

describe("DashboardPage", () => {
  it("shows the KPIs with their hints", async () => {
    renderDashboard();

    expect(await screen.findByRole("heading", { level: 1, name: "Tổng quan" })).toBeInTheDocument();
    expect(screen.getByText("Tình trạng các lớp đang chạy và việc cần xử lý.")).toBeInTheDocument();
    await screen.findByText("Lớp đang chạy");
    expect(kpi("Lớp đang chạy")).toHaveTextContent("Lớp đang chạy21 lớp nháp");
    expect(kpi("Học viên đang học")).toHaveTextContent("Học viên đang học113 chưa đăng nhập lần đầu");
    expect(kpi("Lời mời thất bại")).toHaveTextContent("Lời mời thất bại1Cần gửi lại");
    expect(kpi("Khóa học dùng chặng cũ")).toHaveTextContent("Khóa học dùng chặng cũ1Có thể áp dụng một thao tác");
  });

  it("raises the outdated courses and failed invitations", async () => {
    renderDashboard();
    const warning = (await screen.findByText("Lập trình cơ bản v1", { selector: "strong" })).closest<HTMLElement>(
      '[data-slot="alert"]',
    );
    if (!warning) throw new Error("no outdated alert");
    expect(warning).toHaveTextContent("Lập trình cơ bản v1 vẫn dùng Database v1 trong khi v2 đã phát hành.");
    const [row] = fixtures.dashboard.outdated;
    expect(within(warning).getByRole("link", { name: "Xem và áp dụng" })).toHaveAttribute(
      "href",
      `/admin/stages/${row.stageId}?v=${row.latestVersionId}`,
    );
    expect(screen.getByText("1 lời mời gửi thất bại.").closest('[data-slot="alert"]')).toHaveTextContent(
      "1 lời mời gửi thất bại. Kiểm tra email học viên rồi gửi lại trong trang lớp.",
    );
  });

  it("renders no alert block when nothing needs attention", async () => {
    const { dashboard } = fixtures;
    renderDashboard({
      dashboard: { ...dashboard, outdated: [], hints: { ...dashboard.hints, failedInvites: 0, outdatedCourses: 0 } },
    });
    await screen.findByRole("heading", { name: "Lớp học" });

    expect(document.querySelector('[data-slot="alert"]')).toBeNull();
    expect(kpi("Khóa học dùng chặng cũ")).toHaveTextContent("Mọi khóa học đã cập nhật");
  });

  it("lists every class with its course, members and progress", async () => {
    renderDashboard();
    const classes = card((await screen.findByRole("heading", { name: "Lớp học" })).textContent);

    expect(within(classes).getByRole("link", { name: "Tất cả lớp" })).toHaveAttribute("href", "/admin/classes");
    const [first, , draft] = within(classes).getAllByRole("listitem");
    const basic01 = fixtures.dashboard.classes[0];
    expect(within(first).getByRole("link", { name: "basic01 · Lập trình cơ bản – khóa 1" })).toHaveAttribute(
      "href",
      `/admin/classes/${basic01.id}`,
    );
    expect(first).toHaveTextContent("Lập trình cơ bản v1");
    expect(first).toHaveTextContent("7 học viên");
    expect(within(first).getByText("Đang chạy")).toBeInTheDocument();
    expect(within(first).getByRole("img", { name: "41% hoàn thành" })).toBeInTheDocument();
    expect(within(draft).getByText("Nháp")).toBeInTheDocument();
    expect(within(draft).queryByRole("img")).toBeNull();
  });

  it("lists the eight newest audit lines", async () => {
    const { dashboard } = fixtures;
    const extra = { ...dashboard.recentActivity[0], id: "older", actionLabel: "Tạo chặng" };
    renderDashboard({ dashboard: { ...dashboard, recentActivity: [...dashboard.recentActivity, extra] } });
    const audit = card((await screen.findByRole("heading", { name: "Nhật ký thao tác" })).textContent);

    const items = within(audit).getAllByRole("listitem");
    expect(items).toHaveLength(8);
    expect(items[0]).toHaveTextContent("Phát hành chặng");
    expect(items[0]).toHaveTextContent("Database v2 · Trần Minh Quân");
    expect(within(items[0]).getByText("2 giờ trước")).toHaveAttribute("datetime", dashboard.recentActivity[0].at);
    expect(within(audit).queryByText("Tạo chặng")).toBeNull();
  });

  it("shows empty states without classes or activity", async () => {
    renderDashboard({ dashboard: { ...fixtures.dashboard, classes: [], recentActivity: [] } });
    expect(await screen.findByRole("heading", { name: "Chưa có lớp" })).toBeInTheDocument();
    expect(screen.getByRole("heading", { name: "Chưa có thao tác" })).toBeInTheDocument();
  });

  it("shows a skeleton while loading", async () => {
    let release: () => void = () => undefined;
    const held = new Promise<void>((resolve) => {
      release = resolve;
    });
    renderDashboard({
      overrides: [
        http.get(api("/dashboard"), async () => {
          await held;
          return HttpResponse.json(fixtures.dashboard);
        }),
      ],
    });
    expect(await screen.findByText("Đang tải tổng quan")).toBeInTheDocument();
    release();
    expect(await screen.findByRole("heading", { name: "Lớp học" })).toBeInTheDocument();
    expect(screen.queryByText("Đang tải tổng quan")).toBeNull();
  });

  it("offers a retry when the dashboard cannot load", async () => {
    let fail = true;
    const { user } = renderDashboard({
      overrides: [
        http.get(api("/dashboard"), () =>
          fail ? apiError(500, "INTERNAL", "Có lỗi xảy ra.") : HttpResponse.json(fixtures.dashboard),
        ),
      ],
    });
    expect(await screen.findByRole("alert")).toHaveTextContent("Không tải được tổng quan.");
    fail = false;
    await user.click(screen.getByRole("button", { name: "Thử lại" }));
    expect(await screen.findByRole("heading", { name: "Lớp học" })).toBeInTheDocument();
  });
});
