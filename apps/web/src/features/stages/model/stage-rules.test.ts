import { describe, expect, it } from "vitest";
import { fixtures } from "../test/golden";
import {
  draftExistsMessage,
  lessonPublishBlockers,
  PUBLISH_NEEDS_UNIT,
  stageDeleteBlocker,
  stagePublishBlockers,
} from "./stage-rules";

function fail(message: string): never {
  throw new Error(message);
}

describe("stage rules", () => {
  const draft = fixtures.stageVersionDraftTwoUnits;
  const [first, second] = draft.units;

  it("blocks publishing a draft without units, as the server does", () => {
    expect(stagePublishBlockers({ ...draft, units: [] })).toEqual([PUBLISH_NEEDS_UNIT]);
    expect(PUBLISH_NEEDS_UNIT).toBe("Chặng cần ít nhất một bài học trước khi phát hành.");
  });

  it("names every empty unit of a draft", () => {
    const empty = { ...first, lessonCount: 0, requiredCount: 0 };
    expect(stagePublishBlockers({ ...draft, units: [empty, second] })).toEqual(['Bài học "Nội dung chặng" chưa có học liệu.']);
    expect(stagePublishBlockers({ ...draft, units: [empty, { ...second, lessonCount: 0 }] })).toEqual([
      'Bài học "Nội dung chặng" chưa có học liệu.',
      'Bài học "Nâng cao" chưa có học liệu.',
    ]);
  });

  it("lets a draft whose every unit has a lesson be published, and never blocks other versions", () => {
    expect(stagePublishBlockers(draft)).toEqual([]);
    expect(stagePublishBlockers(fixtures.stageVersionDraft)).toEqual([]);
    expect(stagePublishBlockers({ ...fixtures.stageVersion, units: [] })).toEqual([]);
  });

  describe("quiz and homework", () => {
    const authoring = fixtures.stageVersionDraftAuthoring;
    const quiz = fixtures.lessonQuiz;
    const homework = fixtures.lessonHomework;
    const quizContent = quiz.quiz ?? fail("the quiz golden has a quiz");
    const homeworkContent = homework.homework ?? fail("the homework golden has a homework");
    const [question] = quizContent.questions;
    const unanswered = { ...question, options: question.options.map((o) => ({ ...o, isCorrect: false })) };

    it("lets a complete quiz and homework through", () => {
      expect(lessonPublishBlockers(quiz)).toEqual([]);
      expect(lessonPublishBlockers(homework)).toEqual([]);
      expect(stagePublishBlockers(authoring)).toEqual([]);
    });

    it("blocks a quiz without questions, then counts the questions without a correct answer", () => {
      expect(lessonPublishBlockers({ ...quiz, quiz: { ...quizContent, questions: [] } })).toEqual(['"Kiểm tra JOIN" chưa có câu hỏi.']);
      expect(lessonPublishBlockers({ ...quiz, quiz: { ...quizContent, questions: [question, unanswered, unanswered] } })).toEqual([
        '"Kiểm tra JOIN" có 2 câu chưa đánh dấu đáp án đúng.',
      ]);
    });

    it("blocks a homework without a brief or without criteria", () => {
      const bare = { ...homework, homework: { ...homeworkContent, briefSource: "  \n", criteria: [] } };
      expect(lessonPublishBlockers(bare)).toEqual(['"Bài tập JOIN" chưa có đề bài.', '"Bài tập JOIN" chưa có tiêu chí chấm.']);
    });

    it("lists empty units before lesson reasons, lessons in order, and ignores them once published", () => {
      const lessons = [
        ...authoring.lessons.slice(0, 2),
        { ...quiz, quiz: { ...quizContent, questions: [] } },
        { ...homework, homework: { ...homeworkContent, criteria: [] } },
      ];
      const empty = { ...fixtures.stageVersionDraftTwoUnits.units[1], lessonCount: 0 };
      expect(stagePublishBlockers({ ...authoring, units: [...authoring.units, empty], lessons })).toEqual([
        'Bài học "Nâng cao" chưa có học liệu.',
        '"Kiểm tra JOIN" chưa có câu hỏi.',
        '"Bài tập JOIN" chưa có tiêu chí chấm.',
      ]);
      expect(stagePublishBlockers({ ...authoring, status: "published", lessons })).toEqual([]);
    });
  });

  it("blocks deleting a referenced published version with the courses named", () => {
    expect(stageDeleteBlocker(fixtures.stageVersion)).toBe(
      "Đang được dùng trong Lập trình cơ bản v1. Hãy lưu trữ thay vì xóa.",
    );
    expect(stageDeleteBlocker({ ...fixtures.stageVersion, usedBy: [] })).toBeNull();
    expect(stageDeleteBlocker(fixtures.stageVersionDraft)).toBeNull();
  });

  it("names the existing draft", () => {
    expect(draftExistsMessage(2)).toBe("Chặng đã có bản nháp v2. Mở bản nháp đó để sửa.");
  });
});
