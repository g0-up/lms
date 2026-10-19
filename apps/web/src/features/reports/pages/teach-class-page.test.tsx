import { screen, waitFor, within } from "@testing-library/react";
import { http, HttpResponse } from "msw";
import { describe, expect, it } from "vitest";
import classHomeworkJson from "../../../../../api/internal/features/homework/testdata/class-homework.json";
import classQuizzesJson from "../../../../../api/internal/features/quizzes/testdata/class-quizzes.json";
import classSummaryJson from "../../../../../api/internal/features/summary/testdata/class-summary.json";
import { api, apiError } from "@/shared/test/msw";
import { errorResponse } from "../api/msw-handlers";
import { errors, fixtures, ids } from "../test/golden";
import { gate, polyfillRadix, renderTeach, teachClassPath } from "../test/render-reports";

polyfillRadix();

const classUrl = api("/classes/:classId");
const homeworkUrl = api("/classes/:classId/homework");
const quizzesUrl = api("/classes/:classId/quizzes");

const tabs = () => screen.getByRole("navigation", { name: "Mục của lớp" });

describe("TeachClassPage", () => {
  it("shows the class head and opens on its report, without lifecycle actions", async () => {
    renderTeach(teachClassPath(ids.teachClass));

    expect(
      await screen.findByRole("heading", { level: 1, name: "basic01 · Lập trình cơ bản – khóa 1" }),
    ).toBeInTheDocument();
    expect(screen.getByText("Đang chạy")).toBeInTheDocument();
    expect(screen.getByText("Lập trình cơ bản v1 · 05/10/2026 → 05/10/2026")).toBeInTheDocument();
    const crumbs = screen.getByRole("navigation", { name: "Đường dẫn" });
    expect(within(crumbs).getByRole("link", { name: "Lớp của tôi" })).toHaveAttribute("href", "/teach");

    expect(await screen.findByText(/^7\/7 học viên/)).toBeInTheDocument();
    expect(within(tabs()).getAllByRole("link").map((l) => l.textContent)).toEqual(["Tiến độ", "Tổng kết", "Bài tập", "Quiz"]);
    expect(within(tabs()).getByRole("link", { name: "Tiến độ" })).toHaveAttribute("aria-current", "page");
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

  it("opens the homework tab with grading links in the teacher area", async () => {
    const { user, router } = renderTeach(teachClassPath(ids.teachClass), {
      overrides: [http.get(homeworkUrl, () => HttpResponse.json(classHomeworkJson))],
    });
    await screen.findByText(/^7\/7 học viên/);

    await user.click(within(tabs()).getByRole("link", { name: "Bài tập" }));

    expect(router.state.location.search).toBe("?tab=homework");
    expect(within(tabs()).getByRole("link", { name: "Bài tập" })).toHaveAttribute("aria-current", "page");
    const card = await screen.findByRole("region", { name: "Bài tập: truy vấn SELECT" });
    expect(within(card).getByRole("link", { name: "Chấm bài của Vũ Đức Khang: truy vấn SELECT" })).toHaveAttribute(
      "href",
      "/teach/grading/0199ffff-0000-7000-8000-000000000002",
    );
    expect(screen.queryByText(/^7\/7 học viên/)).toBeNull();
  });

  it("opens the quiz tab with review links in the teacher area", async () => {
    const { user, router } = renderTeach(teachClassPath(ids.teachClass), {
      overrides: [http.get(quizzesUrl, () => HttpResponse.json(classQuizzesJson))],
    });
    await screen.findByText(/^7\/7 học viên/);

    await user.click(within(tabs()).getByRole("link", { name: "Quiz" }));

    expect(router.state.location.search).toBe("?tab=quiz");
    expect(within(tabs()).getByRole("link", { name: "Quiz" })).toHaveAttribute("aria-current", "page");
    const card = await screen.findByRole("region", { name: "Luyện tập: Truy vấn SQL cơ bản" });
    expect(within(card).getByRole("link", { name: "Xem và nhận xét lượt #1 của Nguyễn Hoàng An" })).toHaveAttribute(
      "href",
      "/teach/grading/quiz/0199ffff-0000-7000-8000-000000000005?from=quiz",
    );
    expect(screen.queryByText(/^7\/7 học viên/)).toBeNull();
  });

  it("opens the summary tab with the matrix of the class in the teacher area", async () => {
    const { user, router } = renderTeach(teachClassPath(ids.teachClass), {
      overrides: [http.get(api("/classes/:classId/summary"), () => HttpResponse.json(classSummaryJson))],
    });
    await screen.findByText(/^7\/7 học viên/);

    await user.click(within(tabs()).getByRole("link", { name: "Tổng kết" }));

    expect(router.state.location.search).toBe("?tab=summary");
    expect(within(tabs()).getByRole("link", { name: "Tổng kết" })).toHaveAttribute("aria-current", "page");
    expect(await screen.findByRole("table", { name: "Tổng kết theo chặng lớp basic01" })).toBeInTheDocument();
    expect(screen.getByText("7/7 học viên · Bấm một ô để xem tổng kết chặng và viết nhận xét")).toBeInTheDocument();
    expect(screen.queryByRole("link", { name: "Xóa bộ lọc" })).toBeNull();
  });

  it("shows the API's refusal in the summary tab", async () => {
    const { user } = renderTeach(teachClassPath(ids.teachClass), {
      overrides: [
        http.get(api("/classes/:classId/summary"), () =>
          apiError(403, "FORBIDDEN", "Bạn chỉ xem được lớp mình phụ trách."),
        ),
      ],
    });
    await screen.findByText(/^7\/7 học viên/);

    await user.click(within(tabs()).getByRole("link", { name: "Tổng kết" }));

    expect(await screen.findByRole("alert")).toHaveTextContent("Bạn chỉ xem được lớp mình phụ trách.");
  });

  it("shows the API's refusal in the quiz tab", async () => {
    const { user } = renderTeach(teachClassPath(ids.teachClass), {
      overrides: [http.get(quizzesUrl, () => apiError(403, "FORBIDDEN", "Bạn chỉ xem được lớp mình phụ trách."))],
    });
    await screen.findByText(/^7\/7 học viên/);

    await user.click(within(tabs()).getByRole("link", { name: "Quiz" }));

    expect(await screen.findByRole("alert")).toHaveTextContent("Bạn chỉ xem được lớp mình phụ trách.");
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
