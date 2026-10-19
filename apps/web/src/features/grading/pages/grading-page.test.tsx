import { screen, waitFor, within } from "@testing-library/react";
import { http, HttpResponse } from "msw";
import { describe, expect, it } from "vitest";
import { api, apiError } from "@/shared/test/msw";
import { filterQueue } from "../api/msw-handlers";
import type { Queue } from "../model/schemas";
import { fixtures } from "../test/golden";
import { polyfillRadix, renderGrading } from "../test/render-grading";

polyfillRadix();

const queue = fixtures.queueTeacher;
const [oldest] = queue.items;
const firstQuiz = queue.items.find((i) => i.kind === "quiz");
const homeworkItems = queue.items.filter((i) => i.kind === "homework");

/** Serves `source` filtered as the API does and keeps every query the page sent. */
function recordQueue(source: Queue = queue) {
  const sent: URLSearchParams[] = [];
  const handler = http.get(api("/grading/queue"), ({ request }) => {
    const params = new URL(request.url).searchParams;
    sent.push(params);
    return HttpResponse.json(filterQueue(source, params));
  });
  return { handler, last: () => sent.at(-1) };
}

const searchOf = (router: { state: { location: { search: string } } }) =>
  new URLSearchParams(router.state.location.search);

const oldestAction = { name: `Chấm bài của ${oldest.student.name}: Cài đặt Stack và Queue` };

const queueRegion = () => screen.getByRole("region", { name: "Hàng chờ chấm" });

describe("Grading queue page", () => {
  it("lists the whole queue oldest first with its counts and the overdue badge", async () => {
    renderGrading("/teach/grading");

    expect(await screen.findByRole("heading", { level: 1, name: "Chấm bài" })).toBeInTheDocument();
    expect(
      screen.getByText(
        "Bài tập chờ chấm và lượt Quiz bài tập chờ xác nhận, cũ nhất ở trên. Mục tiêu xử lý trong 3 ngày kể từ lúc nộp.",
      ),
    ).toBeInTheDocument();
    expect(
      await screen.findByText(
        `${String(queue.total)}/${String(queue.total)} mục chờ xử lý · ${String(queue.quizPending)} lượt quiz chờ xác nhận`,
      ),
    ).toBeInTheDocument();
    expect(screen.getByText(`${String(queue.overdueCount)} mục quá 3 ngày`)).toBeInTheDocument();

    const table = within(queueRegion()).getByRole("table", { name: "Hàng chờ chấm, cũ nhất ở trên" });
    const rows = within(table).getAllByRole("row").slice(1);
    expect(rows).toHaveLength(queue.items.length);
    const first = within(rows[0]);
    expect(first.getByText(oldest.student.name)).toBeInTheDocument();
    expect(first.getByText("Quá 3 ngày")).toBeInTheDocument();
    expect(first.getByText("4 ngày 12 giờ")).toBeInTheDocument();
    expect(first.getByText("nộp muộn")).toBeInTheDocument();
    expect(first.getByRole("link", { name: `Chấm bài của ${oldest.student.name}: Cài đặt Stack và Queue` })).toHaveAttribute(
      "href",
      `/teach/grading/${oldest.itemId}`,
    );
  });

  it("sends quiz attempts to their review page with the quiz action", async () => {
    renderGrading("/teach/grading");
    if (!firstQuiz) throw new Error("the golden queue has a quiz attempt");

    const link = await screen.findByRole("link", {
      name: `Xác nhận bài của ${firstQuiz.student.name}: ${firstQuiz.lesson.title}`,
    });
    expect(link).toHaveAttribute("href", `/teach/grading/quiz/${firstQuiz.itemId}`);
    const row = within(link.closest("tr") ?? document.body);
    expect(row.getByText("Quiz bài tập")).toBeInTheDocument();
    expect(row.getByText(new RegExp(`^Lượt #${String(firstQuiz.no)} · ${String(firstQuiz.scorePercent)}%`))).toBeInTheDocument();
  });

  it("applies the filter from the form to the URL and the API, keeping the whole-queue counts", async () => {
    const recorded = recordQueue();
    const { user, router } = renderGrading("/teach/grading", { overrides: [recorded.handler] });
    await screen.findByText(/mục chờ xử lý/);
    expect(recorded.last()?.toString()).toBe("");

    await user.click(screen.getByRole("combobox", { name: "Loại" }));
    await user.click(await screen.findByRole("option", { name: "Bài tập" }));
    await user.click(screen.getByRole("checkbox", { name: "Chỉ mục quá 3 ngày" }));
    await user.click(screen.getByRole("button", { name: "Lọc" }));

    await waitFor(() => {
      expect(searchOf(router).toString()).toBe("type=homework&overdue=1");
    });
    await waitFor(() => {
      expect(recorded.last()?.toString()).toBe("kind=homework&overdue=1");
    });
    const overdueHomework = homeworkItems.filter((i) => i.overdue);
    expect(
      await screen.findByText(`${String(overdueHomework.length)}/${String(queue.total)} mục chờ xử lý`, { exact: false }),
    ).toBeInTheDocument();
    expect(screen.getByText(`${String(queue.overdueCount)} mục quá 3 ngày`)).toBeInTheDocument();
    expect(screen.getByRole("combobox", { name: "Loại" })).toHaveTextContent("Bài tập");
    expect(screen.getByRole("checkbox", { name: "Chỉ mục quá 3 ngày" })).toBeChecked();
  });

  it("filters by class and lesson from the options the API lists", async () => {
    const recorded = recordQueue();
    const { user, router } = renderGrading("/teach/grading", { overrides: [recorded.handler] });
    await screen.findByText(/mục chờ xử lý/);
    const [cls] = queue.filters.classes;

    await user.click(screen.getByRole("combobox", { name: "Lớp" }));
    await user.click(await screen.findByRole("option", { name: cls.code }));
    await user.click(screen.getByRole("combobox", { name: "Bài" }));
    await user.click(await screen.findByRole("option", { name: "Cài đặt Stack và Queue (bài tập)" }));
    await user.click(screen.getByRole("button", { name: "Lọc" }));

    await waitFor(() => {
      expect(searchOf(router).get("lesson")).toBe(oldest.lesson.id);
    });
    expect(searchOf(router).get("class")).toBe(cls.id);
    await waitFor(() => {
      expect(recorded.last()?.get("lessonId")).toBe(oldest.lesson.id);
    });
    expect(recorded.last()?.get("classId")).toBe(cls.id);
  });

  it("drops a malformed filter instead of sending it to the API", async () => {
    const recorded = recordQueue();
    renderGrading("/teach/grading?type=video&class=basic01&lesson=1", { overrides: [recorded.handler] });

    await screen.findByText(/mục chờ xử lý/);
    expect(recorded.last()?.toString()).toBe("");
    expect(screen.queryByRole("link", { name: "Xóa bộ lọc" })).toBeNull();
  });

  it("says when nothing matches the filter and clears it", async () => {
    const recorded = recordQueue();
    const { user, router } = renderGrading(`/teach/grading?type=quiz&lesson=${oldest.lesson.id}`, {
      overrides: [recorded.handler],
    });

    expect(await screen.findByRole("heading", { name: "Không có mục khớp bộ lọc" })).toBeInTheDocument();
    const clear = within(queueRegion()).getAllByRole("link", { name: "Xóa bộ lọc" });
    await user.click(clear[0]);

    await waitFor(() => {
      expect(router.state.location.search).toBe("");
    });
    expect(await screen.findByRole("link", oldestAction)).toBeInTheDocument();
  });

  it("says when the queue is empty, without an overdue badge", async () => {
    renderGrading("/teach/grading", { overrides: [recordQueue(fixtures.emptyQueue).handler] });

    expect(await screen.findByRole("heading", { name: "Không còn bài chờ xử lý" })).toBeInTheDocument();
    expect(screen.getByText("0/0 mục chờ xử lý · 0 lượt quiz chờ xác nhận")).toBeInTheDocument();
    expect(screen.queryByText(/^\d+ mục quá 3 ngày$/)).toBeNull();
  });

  it("lists recent gradings with their verdicts", async () => {
    renderGrading("/teach/grading");

    const heading = await screen.findByRole("heading", { name: "Đã chấm và xác nhận gần đây" });
    const list = await within(heading.closest<HTMLElement>("[data-slot=card]") ?? document.body).findByRole("list");
    const items = within(list).getAllByRole("listitem");
    expect(items).toHaveLength(fixtures.recent.length);
    const [quiz] = fixtures.recent;
    const homeworkAt = fixtures.recent.findIndex((i) => i.kind === "homework");
    const homework = fixtures.recent[homeworkAt];
    expect(within(items[0]).getByRole("link", { name: `${quiz.student.name} · ${quiz.lesson.title}` })).toHaveAttribute(
      "href",
      `/teach/grading/quiz/${quiz.itemId}`,
    );
    expect(within(items[0]).getByText("Chưa đạt")).toBeInTheDocument();
    expect(within(items[0]).getByText(`Quiz · lượt #${String(quiz.no)}`)).toBeInTheDocument();
    expect(within(items[homeworkAt]).getByText("Đạt")).toBeInTheDocument();
    expect(within(items[homeworkAt]).getByText(`Lần nộp #${String(homework.no)}`)).toBeInTheDocument();
    expect(
      within(items[homeworkAt]).getByText(`${String(homework.points)}/${String(homework.maxPoints)} điểm`),
    ).toBeInTheDocument();
  });

  it("tells a homework sent back for rework from a quiz attempt below the threshold", async () => {
    const homework = fixtures.recent.find((i) => i.kind === "homework");
    const quiz = fixtures.recent.find((i) => i.kind === "quiz" && !i.passed);
    if (!homework || !quiz) throw new Error("golden recent items");
    renderGrading("/teach/grading", {
      overrides: [
        http.get(api("/grading/recent"), () => HttpResponse.json({ items: [{ ...homework, passed: false }, quiz] })),
      ],
    });

    const heading = await screen.findByRole("heading", { name: "Đã chấm và xác nhận gần đây" });
    const list = await within(heading.closest<HTMLElement>("[data-slot=card]") ?? document.body).findByRole("list");
    const [reworked, failed] = within(list).getAllByRole("listitem");
    expect(within(reworked).getByText("Cần làm lại")).toBeInTheDocument();
    expect(within(failed).getByText("Chưa đạt")).toBeInTheDocument();
  });

  it("says when nothing was graded yet", async () => {
    renderGrading("/teach/grading", {
      overrides: [http.get(api("/grading/recent"), () => HttpResponse.json({ items: [] }))],
    });

    expect(await screen.findByRole("heading", { name: "Chưa xử lý bài nào" })).toBeInTheDocument();
  });

  it("offers a retry when the queue does not load", async () => {
    let fail = true;
    const { user } = renderGrading("/teach/grading", {
      overrides: [
        http.get(api("/grading/queue"), () =>
          fail ? apiError(500, "INTERNAL", "Lỗi máy chủ.") : HttpResponse.json(queue),
        ),
      ],
    });

    const alert = await within(await screen.findByRole("region", { name: "Hàng chờ chấm" })).findByRole("alert");
    expect(alert).toHaveTextContent("Không tải được hàng chờ.");
    fail = false;
    await user.click(within(alert).getByRole("button", { name: "Thử lại" }));

    expect(await screen.findByRole("link", oldestAction)).toBeInTheDocument();
  });

  it("keeps an admin's links in the admin area", async () => {
    const admin = fixtures.queueAdmin;
    renderGrading("/admin/grading", {
      role: "admin",
      overrides: [recordQueue(admin).handler],
    });
    const [item] = admin.items;

    expect(
      await screen.findByText(`${String(admin.total)}/${String(admin.total)} mục chờ xử lý`, { exact: false }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("link", { name: `Chấm bài của ${item.student.name}: Cài đặt Stack và Queue` }),
    ).toHaveAttribute("href", `/admin/grading/${item.itemId}`);
  });
});
