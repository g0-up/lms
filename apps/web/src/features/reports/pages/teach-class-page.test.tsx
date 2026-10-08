import { screen, waitFor, within } from "@testing-library/react";
import { http, HttpResponse } from "msw";
import { describe, expect, it } from "vitest";
import { api, apiError } from "@/shared/test/msw";
import { errorResponse } from "../api/msw-handlers";
import { errors, fixtures, ids } from "../test/golden";
import { gate, polyfillRadix, renderTeach, teachClassPath } from "../test/render-reports";

polyfillRadix();

const classUrl = api("/classes/:classId");

describe("TeachClassPage", () => {
  it("shows the class head and its report, without tabs or lifecycle actions", async () => {
    renderTeach(teachClassPath(ids.teachClass));

    expect(
      await screen.findByRole("heading", { level: 1, name: "basic01 · Lập trình cơ bản – khóa 1" }),
    ).toBeInTheDocument();
    expect(screen.getByText("Đang chạy")).toBeInTheDocument();
    expect(screen.getByText("Lập trình cơ bản v1 · 05/10/2026 → 05/10/2026")).toBeInTheDocument();
    const crumbs = screen.getByRole("navigation", { name: "Đường dẫn" });
    expect(within(crumbs).getByRole("link", { name: "Lớp của tôi" })).toHaveAttribute("href", "/teach");

    expect(await screen.findByText(/^7\/7 học viên/)).toBeInTheDocument();
    expect(screen.queryByRole("navigation", { name: "Mục của lớp" })).toBeNull();
    expect(screen.queryByRole("button", { name: /Kích hoạt lớp|Kết thúc lớp/ })).toBeNull();
    await waitFor(() => { expect(document.title).toBe("basic01 · GoUp LMS"); });
  });

  it("keeps the teacher's filter in the query string without a tab", async () => {
    const { user, router } = renderTeach(teachClassPath(ids.teachClass));
    await screen.findByText(/^7\/7 học viên/);

    await user.type(screen.getByLabelText("% toàn khóa dưới"), "50");
    await user.click(screen.getByRole("button", { name: "Lọc" }));

    await waitFor(() => { expect(router.state.location.search).toBe("?below=50"); });
    expect(screen.getByRole("link", { name: "Xóa bộ lọc" })).toHaveAttribute("href", teachClassPath(ids.teachClass));
  });

  it("sends a teacher back to their classes when the class is not theirs", async () => {
    const { router } = renderTeach(teachClassPath("01990000-0000-7000-8000-000000000999"));

    expect(await screen.findByText("Bạn chỉ xem được lớp mình phụ trách.")).toBeInTheDocument();
    await waitFor(() => { expect(router.state.location.pathname).toBe("/teach"); });
    expect(await screen.findByRole("heading", { level: 1, name: "Lớp của tôi" })).toBeInTheDocument();
    // Replaced, not pushed: Back must not reopen the forbidden page.
    expect(router.state.historyAction).toBe("REPLACE");
  });

  it("treats a class that does not exist the same way", async () => {
    const { router } = renderTeach(teachClassPath(ids.teachClass), {
      overrides: [http.get(classUrl, () => errorResponse(404, errors.notFound))],
    });

    expect(await screen.findByText("Bạn chỉ xem được lớp mình phụ trách.")).toBeInTheDocument();
    await waitFor(() => { expect(router.state.location.pathname).toBe("/teach"); });
  });

  it("shows a loading state, then offers a retry on other failures", async () => {
    const hold = gate();
    let fail = true;
    const { user } = renderTeach(teachClassPath(ids.teachClass), {
      overrides: [
        http.get(classUrl, async () => {
          await hold.wait();
          return fail ? apiError(500, "INTERNAL", "Lỗi máy chủ") : HttpResponse.json(fixtures.teachClass);
        }),
      ],
    });

    expect((await screen.findByText("Đang tải lớp")).parentElement).toHaveAttribute("aria-busy", "true");
    hold.open();
    const alert = await screen.findByRole("alert");
    expect(alert).toHaveTextContent("Lỗi máy chủ");

    fail = false;
    await user.click(within(alert).getByRole("button", { name: "Thử lại" }));
    expect(await screen.findByRole("heading", { level: 1, name: /^basic01/ })).toBeInTheDocument();
  });
});
