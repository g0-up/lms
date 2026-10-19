import { describe, expect, it } from "vitest";
import { fmtDateTime } from "@/shared/lib/format";
import { fixtures } from "../test/golden";
import { itemDetail } from "./item-detail";
import type { QuizFacts, SummaryItem } from "./schemas";

const items = fixtures.learner.stages.flatMap((s) => s.units.flatMap((u) => u.items));
const item = (title: string): SummaryItem => {
  const found = items.find((i) => i.title === title);
  if (!found) throw new Error(`no item ${title}`);
  return found;
};
const quiz = (title: string, facts: Partial<QuizFacts>): SummaryItem => {
  const base = item(title);
  if (!base.quiz) throw new Error(`${title} is not a quiz`);
  return { ...base, quiz: { ...base.quiz, ...facts } };
};
// The golden's times are all the same instant; dates are shown in the viewer's time zone.
const at = fmtDateTime("2026-10-05T08:00:00Z");

describe("itemDetail of a quiz", () => {
  it("gives a practice quiz its best and latest score and the attempts without a limit", () => {
    expect(itemDetail(item("Luyện tập: Truy vấn SQL cơ bản"))).toBe("Tốt nhất 75% (3/4) · 1 lượt · Gần nhất 75% (3/4)");
    expect(
      itemDetail(
        quiz("Luyện tập: Stack, Queue và Hash", { best: { score: 5, max: 5, percent: 100 }, latest: { score: 3, max: 5, percent: 60, reviewStatus: null } }),
      ),
    ).toBe("Tốt nhất 100% (5/5) · 2 lượt · Gần nhất 60% (3/5)");
  });

  it("gives an assessment its best score, the attempts used of the limit and the review state", () => {
    expect(itemDetail(item("Kiểm tra cuối chặng Database"))).toBe("Tốt nhất 80% (4/5) · 1/2 lượt · Đã xác nhận");
    expect(itemDetail(item("Kiểm tra cuối chặng Go"))).toBe("Tốt nhất 60% (3/5) · 1/2 lượt · Chờ xác nhận");
  });

  it("counts an open attempt against the limit", () => {
    expect(itemDetail(quiz("Kiểm tra cuối chặng Go", { attemptsUsed: 2, open: true }))).toBe(
      "Tốt nhất 60% (3/5) · 2/2 lượt · Chờ xác nhận",
    );
  });

  it("says a quiz is not done yet, with its limit when it has one", () => {
    expect(itemDetail(item("Luyện tập: HTML ngữ nghĩa và CSS layout"))).toBe("Chưa làm");
    expect(itemDetail(item("Kiểm tra cuối chặng ReactJS"))).toBe("Chưa làm · 2 lượt");
    expect(itemDetail(quiz("Kiểm tra cuối chặng ReactJS", { attemptsUsed: 1, open: true }))).toBe("Đang làm · 2 lượt");
  });

  it("falls back to the API's sentence without the quiz facts", () => {
    expect(itemDetail({ ...item("Kiểm tra cuối chặng Database"), quiz: undefined })).toBe(
      "Tốt nhất 80% · 1/2 lượt · Đã xác nhận",
    );
  });
});

describe("itemDetail of a homework", () => {
  it("gives the latest submission with its lateness and score", () => {
    expect(itemDetail(item("Bài tập: Cài đặt Stack và Queue"))).toBe(`Lần nộp #2 · ${at} · nộp muộn · 94/100 điểm`);
  });

  it("leaves out the score of a submission waiting for grading", () => {
    const base = item("Bài tập: Cài đặt Stack và Queue");
    const latest = { attemptNo: 3, status: "pending" as const, isLate: false, submittedAt: "2026-10-05T08:00:00Z", total: null, max: null };

    expect(itemDetail({ ...base, homework: { dueAt: null, submissionCount: 3, latest } })).toBe(`Lần nộp #3 · ${at}`);
  });

  it("gives the deadline of a homework not submitted, or says it has none", () => {
    const base = item("Bài tập: Đọc CSV và thống kê");

    expect(itemDetail(base, new Date("2026-10-03T08:00:00Z"))).toBe(`Hạn ${at} · còn 2 ngày`);
    expect(itemDetail(base, new Date("2026-10-05T11:00:00Z"))).toBe(`Hạn ${at} · quá hạn 3 giờ`);
    expect(itemDetail({ ...base, homework: { dueAt: null, submissionCount: 0, latest: null } })).toBe(
      "Chưa nộp · chưa đặt hạn",
    );
  });

  it("falls back to the API's sentence without the homework facts", () => {
    expect(itemDetail({ ...item("Bài tập: Đọc CSV và thống kê"), homework: undefined })).toBe("Chưa nộp");
  });
});

describe("itemDetail of a video or reading", () => {
  it("says when it was ticked, else first opened, else that it was never opened", () => {
    expect(itemDetail(item("Giới thiệu SQL"))).toBe(`Tích hoàn thành ${at}`);
    expect(itemDetail(item("Hooks"))).toBe(`Mở lần đầu ${at}`);
    expect(itemDetail(item("HTML ngữ nghĩa"))).toBe("Chưa mở");
  });
});
