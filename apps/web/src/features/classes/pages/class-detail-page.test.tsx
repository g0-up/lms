import { screen, waitFor, within } from "@testing-library/react";
import { http, HttpResponse } from "msw";
import { beforeAll, describe, expect, it } from "vitest";
import { api } from "@/shared/test/msw";
import { errorResponse, seedDb } from "../api/msw-handlers";
import { errors, ids } from "../test/golden";
import { classPath, gate, polyfillRadix, renderClasses } from "../test/render-classes";

polyfillRadix();

// The route is lazy: load it once up front so the first test's timeouts do not include the import.
beforeAll(async () => {
  await import("./class-detail-page");
});

const tabs = () => screen.getByRole("navigation", { name: "Mục của lớp" });
const tab = (name: RegExp) => within(tabs()).getByRole("link", { name });

describe("class detail page", () => {
  it("shows the class head, its lifecycle action and the students tab by default", async () => {
    renderClasses(classPath(ids.active));

    expect(await screen.findByRole("heading", { level: 1, name: "basic01 · Lập trình cơ bản – khóa 1" })).toBeInTheDocument();
    expect(screen.getByText("Lập trình cơ bản v1 · 05/10/2026 → 01/02/2027 · Giảng viên Lê Thu Hương")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Lớp học" })).toHaveAttribute("href", "/admin/classes");
    expect(screen.getByRole("button", { name: "Kết thúc lớp" })).toBeInTheDocument();
    expect(document.title).toContain("basic01");

    expect(tab(/^Học viên/)).toHaveAttribute("aria-current", "page");
    expect(tab(/^Học viên/)).toHaveTextContent("5");
    expect(tab(/^Tiến độ/)).not.toHaveAttribute("aria-current");
    expect(await screen.findByRole("table", { name: "Học viên lớp basic01" })).toBeInTheDocument();
  });

  it("falls back to the students tab for an unknown tab", async () => {
    renderClasses(classPath(ids.active, "nope"));

    expect(await screen.findByRole("table", { name: "Học viên lớp basic01" })).toBeInTheDocument();
    expect(tab(/^Học viên/)).toHaveAttribute("aria-current", "page");
  });

  it("switches tabs through the query string and keeps the tab when filtering the report", async () => {
    const { user, router } = renderClasses(classPath(ids.active));
    await screen.findByRole("table", { name: "Học viên lớp basic01" });

    await user.click(tab(/^Tiến độ/));
    expect(router.state.location.search).toBe("?tab=report");
    expect(await screen.findByRole("table", { name: "Tiến độ học viên lớp basic01" })).toBeInTheDocument();
    expect(screen.getByText(/^5\/5 học viên/)).toBeInTheDocument();
    expect(tab(/^Tiến độ/)).toHaveAttribute("aria-current", "page");

    await user.click(screen.getByRole("checkbox", { name: "Chưa đăng nhập" }));
    await user.click(screen.getByRole("button", { name: "Lọc" }));
    await waitFor(() => {
      const search = new URLSearchParams(router.state.location.search);
      expect(search.get("tab")).toBe("report");
      expect(search.get("notlogged")).toBe("1");
    });

    await user.click(tab(/^Cài đặt/));
    expect(router.state.location.search).toBe("?tab=settings");
    expect(await screen.findByRole("form", { name: "Cài đặt lớp" })).toBeInTheDocument();
  });

  it("shows a loading state, then the error with a retry", async () => {
    const hold = gate();
    const { user } = renderClasses(classPath(ids.active), {
      overrides: [
        http.get(
          api("/classes/:classId"),
          async () => {
            await hold.wait();
            return HttpResponse.json({ error: { code: "INTERNAL", message: "Lỗi máy chủ." } }, { status: 500 });
          },
          { once: true },
        ),
      ],
    });

    expect(await screen.findByText("Đang tải lớp")).toBeInTheDocument();
    hold.open();
    expect(await screen.findByRole("alert")).toHaveTextContent("Lỗi máy chủ.");
    expect(screen.getByRole("link", { name: "Lớp học" })).toHaveAttribute("href", "/admin/classes");

    await user.click(screen.getByRole("button", { name: "Thử lại" }));
    expect(await screen.findByRole("heading", { level: 1, name: /^basic01/ })).toBeInTheDocument();
  });

  it("says when the class does not exist", async () => {
    renderClasses(classPath("0199eeee-0000-7000-8000-999999999999"));

    expect(await screen.findByRole("alert")).toHaveTextContent("Không tìm thấy lớp.");
  });

  it("activates a draft class after confirmation", async () => {
    const { user, db } = renderClasses(classPath(ids.draft));

    await user.click(await screen.findByRole("button", { name: "Kích hoạt lớp" }));
    const dialog = await screen.findByRole("dialog", { name: "Kích hoạt lớp basic04?" });
    expect(dialog).toHaveTextContent("0 học viên sẽ bắt đầu học");
    await user.click(within(dialog).getByRole("button", { name: "Kích hoạt" }));

    expect(await screen.findByText("Lớp basic04 đang chạy.")).toBeInTheDocument();
    expect(await screen.findByRole("button", { name: "Kết thúc lớp" })).toBeInTheDocument();
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
    expect(db.classes.find((c) => c.id === ids.draft)?.status).toBe("active");
  });

  it("ends an active class: no lifecycle action and no invitation afterwards", async () => {
    const { user } = renderClasses(classPath(ids.active));

    await user.click(await screen.findByRole("button", { name: "Kết thúc lớp" }));
    const dialog = await screen.findByRole("dialog", { name: "Kết thúc lớp basic01?" });
    expect(dialog).toHaveTextContent("Không hoàn tác được.");
    await user.click(within(dialog).getByRole("button", { name: "Kết thúc lớp" }));

    expect(await screen.findByText("Lớp basic01 đã kết thúc.")).toBeInTheDocument();
    await waitFor(() => { expect(screen.queryByRole("button", { name: "Kết thúc lớp" })).not.toBeInTheDocument(); });
    expect(screen.queryByRole("button", { name: "Kích hoạt lớp" })).not.toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Mời học viên" })).toHaveAttribute("aria-disabled", "true");
  });

  it("reloads the class when it already moved on elsewhere", async () => {
    const { user, db } = renderClasses(classPath(ids.draft));

    await user.click(await screen.findByRole("button", { name: "Kích hoạt lớp" }));
    db.classes = db.classes.map((c) => (c.id === ids.draft ? { ...c, status: "active" } : c));
    await user.click(within(await screen.findByRole("dialog")).getByRole("button", { name: "Kích hoạt" }));

    expect(await screen.findByText(errors.invalidTransition.error.message)).toBeInTheDocument();
    expect(await screen.findByRole("button", { name: "Kết thúc lớp" })).toBeInTheDocument();
  });

  it("shows no lifecycle action for an ended class", async () => {
    const db = seedDb();
    db.classes = db.classes.map((c) => (c.id === ids.active ? { ...c, status: "ended" } : c));
    renderClasses(classPath(ids.active), {
      db,
      overrides: [http.post(api("/classes/:classId/end"), () => errorResponse(409, errors.invalidTransition))],
    });

    expect(await screen.findByRole("heading", { level: 1, name: /^basic01/ })).toBeInTheDocument();
    expect(screen.getByText("Đã kết thúc")).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Kết thúc lớp" })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Kích hoạt lớp" })).not.toBeInTheDocument();
  });
});
