import { screen, waitFor, within } from "@testing-library/react";
import { http, HttpResponse } from "msw";
import { describe, expect, it } from "vitest";
import { fmtDateTime } from "@/shared/lib/format";
import { api, apiError } from "@/shared/test/msw";
import { NOT_MEMBER_MESSAGE } from "../model/messages";
import type { LearnerSummary } from "../model/schemas";
import { fixtures } from "../test/golden";
import { learnSummaryPath, renderLearnSummary } from "../test/render-summary";

const summaryUrl = api("/me/classes/:classId/summary");
const { learner } = fixtures;
const classPath = `/learn/classes/${learner.class.id}`;

/** The card of the stage whose heading is `name` ("2. Data structure"). */
const stageCard = (name: string) => {
  const card = screen.getByRole("heading", { level: 2, name }).closest<HTMLElement>('[data-slot="card"]');
  if (!card) throw new Error(`no card for ${name}`);
  return within(card);
};

describe("learn summary page", () => {
  it("shows the course progress and every stage with its badge", async () => {
    renderLearnSummary();

    expect(await screen.findByRole("heading", { level: 1, name: "Tổng kết khóa học" })).toBeInTheDocument();
    expect(screen.getByText(learner.class.name)).toBeInTheDocument();
    const crumbs = screen.getByRole("navigation", { name: "Đường dẫn" });
    expect(within(crumbs).getByRole("link", { name: "Lớp của tôi" })).toHaveAttribute("href", "/learn");
    expect(within(crumbs).getByRole("link", { name: "basic01" })).toHaveAttribute("href", classPath);
    expect(within(crumbs).getByText("Tổng kết")).toHaveAttribute("aria-current", "page");
    await waitFor(() => { expect(document.title).toContain("Tổng kết basic01"); });

    expect(screen.getByText("16/25 học liệu bắt buộc. Học liệu không bắt buộc không tính vào %.")).toBeInTheDocument();
    expect(screen.getByRole("img", { name: "64% hoàn thành" })).toBeInTheDocument();
    expect(screen.getAllByRole("heading", { level: 2 }).map((h) => h.textContent)).toEqual([
      "Toàn khóa",
      "1. Database",
      "2. Data structure",
      "3. Golang basic",
      "4. ReactJS basic",
      "5. HTML CSS JS",
    ]);
    expect(stageCard("1. Database").getByText("4/4 học liệu bắt buộc")).toBeInTheDocument();
    expect(stageCard("1. Database").getByText("Hoàn thành chặng")).toBeInTheDocument();
    expect(stageCard("3. Golang basic").getByText("3/5 học liệu bắt buộc")).toBeInTheDocument();
    expect(stageCard("3. Golang basic").getByText("Đang học")).toBeInTheDocument();
  });

  it("sums up the whole course stage by stage, with scores but no course-wide total", async () => {
    renderLearnSummary();
    await screen.findByRole("heading", { level: 1, name: "Tổng kết khóa học" });

    expect(screen.getByText("chặng hoàn thành").parentElement).toHaveTextContent("2/5 chặng hoàn thành");
    const table = within(screen.getByRole("table", { name: "Tổng kết từng chặng" }));
    expect(table.getAllByRole("columnheader").map((th) => th.textContent)).toEqual([
      "Chặng",
      "Trạng thái",
      "Còn thiếu",
      "Quiz bài tập",
      "Homework",
    ]);
    const cells = (name: string) => {
      const row = table.getByRole("cell", { name }).closest("tr");
      if (!row) throw new Error(`no row for ${name}`);
      return within(row)
        .getAllByRole("cell")
        .map((td) => td.textContent);
    };
    expect(cells("1. Database")).toEqual(["1. Database", "Hoàn thành chặng", "—", "Tốt nhất 80% · Đạt", "—"]);
    expect(cells("2. Data structure")).toEqual([
      "2. Data structure",
      "Hoàn thành chặng",
      "—",
      "Tốt nhất 100% · Đạt",
      "94/100 · Đạt",
    ]);
    expect(cells("3. Golang basic")).toEqual([
      "3. Golang basic",
      "Đang học",
      "còn 2 học liệu",
      "Tốt nhất 60% · Chưa đạt",
      "Chưa nộp",
    ]);
    expect(cells("4. ReactJS basic")).toEqual(["4. ReactJS basic", "Đang học", "còn 2 học liệu", "Chưa làm", "—"]);
    expect(cells("5. HTML CSS JS")).toEqual(["5. HTML CSS JS", "Đang học", "còn 5 học liệu", "Chưa làm", "—"]);
    expect(screen.queryByText(/điểm tổng|tổng điểm/i)).toBeNull();
  });

  it("flags a stage's overdue work in the course table and names each score when a stage has several", async () => {
    const [, , golang] = learner.course.stages;
    const extra = { ...golang.assessments[0], lessonId: "0199ffff-0000-7000-8000-0000000000c1", title: "Kiểm tra giữa chặng Go", bestPercent: 90, passed: true };
    const summary: LearnerSummary = {
      ...learner,
      stages: learner.stages.map((s) => (s.stageVersionId === golang.stageVersionId ? { ...s, overdue: true } : s)),
      course: {
        ...learner.course,
        stages: learner.course.stages.map((s) =>
          s === golang ? { ...s, assessments: [...s.assessments, extra] } : s,
        ),
      },
    };
    renderLearnSummary(learnSummaryPath(), { overrides: [http.get(summaryUrl, () => HttpResponse.json(summary))] });
    await screen.findByRole("heading", { level: 1, name: "Tổng kết khóa học" });

    const table = within(screen.getByRole("table", { name: "Tổng kết từng chặng" }));
    const row = within(table.getByRole("cell", { name: "3. Golang basic" }).closest("tr") as HTMLElement);
    expect(row.getByText("Có bài quá hạn")).toBeInTheDocument();
    expect(row.getAllByRole("listitem").map((li) => li.textContent)).toEqual([
      "Kiểm tra cuối chặng Go: Tốt nhất 60% · Chưa đạt",
      "Kiểm tra giữa chặng Go: Tốt nhất 90% · Đạt",
    ]);
  });

  it("lists what an incomplete stage still misses, and nothing for a complete one", async () => {
    renderLearnSummary();
    await screen.findByRole("heading", { level: 1, name: "Tổng kết khóa học" });

    const go = stageCard("3. Golang basic");
    const missing = go.getByRole("heading", { level: 3, name: "Còn thiếu" }).nextElementSibling as HTMLElement;
    expect(within(missing).getAllByRole("listitem").map((li) => li.textContent)).toEqual([
      "Bài tập: Đọc CSV và thống kê · Kiểm tra cuối chặng",
      "Kiểm tra cuối chặng Go · Kiểm tra cuối chặng",
    ]);
    expect(stageCard("2. Data structure").queryByRole("heading", { name: "Còn thiếu" })).toBeNull();
  });

  it("groups lessons by bài học, links them and marks the optional ones", async () => {
    renderLearnSummary();
    await screen.findByRole("heading", { level: 1, name: "Tổng kết khóa học" });

    const db = stageCard("1. Database");
    expect(db.getAllByRole("heading", { level: 3 }).map((h) => h.textContent)).toEqual([
      "Nhập môn SQL",
      "Thiết kế bảng",
      "Kiểm tra cuối chặng",
      "Nhận xét của giảng viên",
    ]);
    const lesson = learner.stages[0].units[0].items[0];
    expect(db.getByRole("link", { name: "Giới thiệu SQL" })).toHaveAttribute("href", `${classPath}/lessons/${lesson.lessonId}`);
    const optional = db.getByRole("link", { name: "Đọc thêm: chỉ mục" }).closest("li") as HTMLElement;
    expect(within(optional).getByText("Không bắt buộc")).toBeInTheDocument();
  });

  it("shows the teacher's review of a stage, or says there is none yet", async () => {
    renderLearnSummary();
    await screen.findByRole("heading", { level: 1, name: "Tổng kết khóa học" });

    const review = learner.stages[1].review;
    const ds = stageCard("2. Data structure");
    expect(ds.getByText(review?.body ?? "")).toBeInTheDocument();
    expect(ds.getByText(`Lê Thu Hương · ${fmtDateTime(review?.updatedAt ?? "")}`)).toBeInTheDocument();
    expect(stageCard("1. Database").getByText("Chưa có nhận xét của giảng viên cho chặng này.")).toBeInTheDocument();
  });

  it("does not link lessons of a draft class", async () => {
    const draft: LearnerSummary = { ...learner, class: { ...learner.class, status: "draft" } };
    renderLearnSummary(learnSummaryPath(), { overrides: [http.get(summaryUrl, () => HttpResponse.json(draft))] });
    await screen.findByRole("heading", { level: 1, name: "Tổng kết khóa học" });

    expect(stageCard("1. Database").getByText("Giới thiệu SQL")).toBeInTheDocument();
    expect(screen.queryByRole("link", { name: "Giới thiệu SQL" })).toBeNull();
  });

  it.each([
    [403, "FORBIDDEN", "Bạn không còn học lớp này."],
    [404, "NOT_FOUND", "Không tìm thấy lớp."],
  ] as const)("sends a student back to their classes on a %i", async (status, code, message) => {
    const { router } = renderLearnSummary(learnSummaryPath(), {
      overrides: [http.get(summaryUrl, () => apiError(status, code, message))],
    });

    expect(await screen.findByRole("heading", { level: 1, name: "Lớp của tôi" })).toBeInTheDocument();
    expect(router.state.location.pathname).toBe("/learn");
    expect(await screen.findByText(NOT_MEMBER_MESSAGE)).toBeInTheDocument();
  });

  it("shows any other failure with a retry", async () => {
    let fail = true;
    const { user } = renderLearnSummary(learnSummaryPath(), {
      overrides: [
        http.get(summaryUrl, () =>
          fail ? apiError(500, "INTERNAL", "Máy chủ đang bận, thử lại sau.") : HttpResponse.json(learner),
        ),
      ],
    });

    expect(await screen.findByRole("alert")).toHaveTextContent("Máy chủ đang bận, thử lại sau.");
    fail = false;
    await user.click(screen.getByRole("button", { name: "Thử lại" }));
    expect(await screen.findByRole("heading", { level: 1, name: "Tổng kết khóa học" })).toBeInTheDocument();
  });
});
