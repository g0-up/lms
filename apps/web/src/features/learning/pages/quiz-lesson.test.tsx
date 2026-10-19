import { act, screen, within } from "@testing-library/react";
import { http, HttpResponse } from "msw";
import { afterEach, beforeAll, describe, expect, it, vi } from "vitest";
import { api, apiError } from "@/shared/test/msw";
import { closeAttempt, quizState, type QuizState } from "../api/msw-handlers";
import { fixtures, ids, openAssessment } from "../test/golden";
import { gate, lessonPath, renderLearn } from "../test/render-learn";

// The route is lazy: load it once up front so the first test's timeouts do not include the import.
beforeAll(async () => {
  await import("./lesson-page");
});

const practicePath = lessonPath(ids.active, ids.quizPractice);
const assessmentPath = lessonPath(ids.active, ids.quizAssessment);
const answerUrl = api("/me/attempts/:attemptId/answers/:questionId");
const submitUrl = api("/me/attempts/:attemptId/submit");
const startUrl = api("/me/classes/:classId/lessons/:lessonId/attempts");

const [q1, q2, q3] = fixtures.openPractice.questions;
const SAVED_AT = /^Đã lưu \d{2}:\d{2}$/;
const TOAST = "[data-sonner-toast] *";
const TIME_UP = "Hết giờ: hệ thống đã tự nộp bài với các câu đã lưu.";
const conflict = (message: string) => apiError(409, "INVALID_TRANSITION", message);
/** The status bar's position, whose number sits in its own element. */
const position = (text: string) =>
  screen.getByText((_, el) => el?.tagName === "SPAN" && el.textContent === text);

async function renderQuiz(path: string, quiz: QuizState = quizState(), overrides: Parameters<typeof renderLearn>[1] = {}) {
  const view = renderLearn(path, { quiz, ...overrides });
  // The first render of a run loads the lazy page modules, which can outlast the default second.
  await screen.findByRole("heading", { level: 1 }, { timeout: 5_000 });
  return view;
}

/** The practice quiz with its attempt open on the server, as left by an earlier visit. */
function practiceOpen(attempt = fixtures.openPractice): QuizState {
  return quizState({
    overviews: { [ids.quizPractice]: fixtures.practiceOverviewOpen },
    attempts: { [attempt.id]: attempt },
  });
}

/** The assessment with its attempt open and `minutesLeft` on the clock. */
function assessmentOpen(minutesLeft = 10): QuizState {
  const attempt = openAssessment(minutesLeft);
  return quizState({
    overviews: { [ids.quizAssessment]: fixtures.assessmentOverviewOpen },
    attempts: { [attempt.id]: attempt },
  });
}

/** The assessment result as the server closes it at the deadline. */
const timedOut = { ...fixtures.resultAssessmentPending, autoSubmitted: true };

const questionMap = () => within(screen.getByRole("navigation", { name: "Chuyển nhanh đến câu hỏi" }));
const sideList = () => screen.queryByRole("complementary", { name: /^Học liệu chặng/ });

afterEach(() => {
  window.sessionStorage.clear();
  vi.useRealTimers();
  vi.unstubAllGlobals();
});

describe("Quiz lesson: start screen", () => {
  it("shows a practice quiz's facts and how an attempt works, next to the stage's lessons", async () => {
    await renderQuiz(practicePath);

    expect(await screen.findByText("Quiz học liệu · luyện tập, làm bao nhiêu lần cũng được")).toBeInTheDocument();
    expect(screen.getByText("3 câu · 6 điểm")).toBeInTheDocument();
    expect(screen.queryByText("Ngưỡng đạt")).toBeNull();
    expect(
      screen.getByText(
        "Mỗi lần chỉ hiện một câu và câu trả lời được lưu ngay khi chọn. Nộp xong, bạn xem được đáp án và giải thích từng câu.",
      ),
    ).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Bắt đầu làm bài" })).not.toHaveAttribute("aria-disabled");
    expect(screen.queryByRole("button", { name: "Đã học xong" })).toBeNull();
    expect(screen.getByRole("link", { name: "Học liệu trước" })).toBeInTheDocument();
    expect(sideList()).toBeInTheDocument();
  });

  it("shows an assessment's limits, the best result and past attempts with their review state", async () => {
    await renderQuiz(
      assessmentPath,
      quizState({ overviews: { [ids.quizAssessment]: fixtures.assessmentOverviewPassed } }),
    );

    expect(
      await screen.findByText("Quiz bài tập · tính vào tiến độ, giảng viên xác nhận từng lượt"),
    ).toBeInTheDocument();
    expect(screen.getByText("3 câu · 10 điểm")).toBeInTheDocument();
    expect(screen.getByText("70%")).toBeInTheDocument();
    expect(screen.getByText("10 phút mỗi lượt")).toBeInTheDocument();
    expect(screen.getByText("1/2 đã dùng · tính lượt tốt nhất")).toBeInTheDocument();
    expect(
      screen.getByText("Bạn đã đạt quiz bài tập này với 70%. Bạn vẫn có thể làm thêm; kết quả tính theo lượt tốt nhất."),
    ).toBeInTheDocument();
    expect(
      screen.getByText(
        "Đồng hồ 10 phút bắt đầu khi bạn bấm làm bài và vẫn chạy nếu bạn rời trang. Mỗi lần chỉ hiện một câu; câu trả lời được lưu ngay khi chọn; hết giờ, bài tự nộp.",
      ),
    ).toBeInTheDocument();

    const history = within(screen.getByRole("region", { name: "Các lượt đã nộp" }));
    const [row] = history.getAllByRole("listitem");
    expect(within(row).getByRole("link", { name: "Lượt 1" })).toHaveAttribute(
      "href",
      `${assessmentPath}?result=${ids.assessmentAttempt}`,
    );
    expect(within(row).getByText("7/10 · 70%")).toBeInTheDocument();
    expect(within(row).getByText("Đạt")).toBeInTheDocument();
    expect(within(row).getByText("Đã xác nhận")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Làm lượt mới" })).toBeInTheDocument();
  });

  it("blocks a new attempt once every attempt is used, saying why, and never asks the server", async () => {
    const quiz = quizState({ overviews: { [ids.quizAssessment]: fixtures.assessmentOverviewOut } });
    const { user } = await renderQuiz(assessmentPath, quiz);

    expect(
      await screen.findByText(
        "Bạn đã dùng hết 2 lượt mà chưa đạt. Liên hệ giảng viên nếu cần cấp thêm lượt.",
      ),
    ).toBeInTheDocument();
    expect(screen.queryByText(/Đồng hồ 10 phút/)).toBeNull();

    const [latest, first] = within(screen.getByRole("region", { name: "Các lượt đã nộp" })).getAllByRole("listitem");
    expect(within(latest).getByText("Chưa đạt")).toBeInTheDocument();
    expect(within(latest).getByText("Chờ xác nhận")).toBeInTheDocument();
    expect(within(first).getByText("tự nộp khi hết giờ")).toBeInTheDocument();
    expect(within(first).queryByText("Đã có lượt mới hơn")).toBeNull();

    const start = screen.getByRole("button", { name: "Làm lượt mới" });
    expect(start).toHaveAttribute("aria-disabled", "true");
    expect(start).toHaveAttribute("title", "Bạn đã dùng hết 2 lượt");
    await user.click(start);
    expect(await screen.findByText("Bạn đã dùng hết 2 lượt", { selector: TOAST })).toBeInTheDocument();
    expect(quiz.attempts).toEqual({});
  });

  it("opens a new attempt again once the teacher grants more, saying they were granted", async () => {
    const granted = {
      ...fixtures.assessmentOverviewOut,
      limit: 3,
      granted: 1,
      canStart: true,
      blockedReason: undefined,
    };
    await renderQuiz(assessmentPath, quizState({ overviews: { [ids.quizAssessment]: granted } }));

    expect(await screen.findByText("2/3 đã dùng · đã được cấp thêm · tính lượt tốt nhất")).toBeInTheDocument();
    expect(screen.queryByText(/Bạn đã dùng hết/)).toBeNull();
    expect(screen.getByRole("button", { name: "Làm lượt mới" })).not.toHaveAttribute("aria-disabled");
  });

  it("shows the server's refusal to start and the start screen it sees now", async () => {
    const quiz = quizState();
    const { user } = await renderQuiz(assessmentPath, quiz, {
      overrides: [
        http.post(startUrl, () => {
          quiz.overviews[ids.quizAssessment] = fixtures.assessmentOverviewOut;
          return conflict("Bạn đã dùng hết 2 lượt làm bài.");
        }),
      ],
    });

    await user.click(await screen.findByRole("button", { name: "Bắt đầu làm bài" }));

    expect(await screen.findByText("Bạn đã dùng hết 2 lượt làm bài.")).toBeInTheDocument();
    expect(
      await screen.findByText("Bạn đã dùng hết 2 lượt mà chưa đạt. Liên hệ giảng viên nếu cần cấp thêm lượt."),
    ).toBeInTheDocument();
    expect(screen.queryByRole("navigation", { name: "Chuyển nhanh đến câu hỏi" })).toBeNull();
  });
});

describe("Quiz lesson: focus mode", () => {
  it("starts an attempt on one question, without the side list, focused on the question", async () => {
    const { user } = await renderQuiz(practicePath);

    await user.click(await screen.findByRole("button", { name: "Bắt đầu làm bài" }));

    const heading = await screen.findByRole("heading", { level: 2, name: "Khóa chính dùng để làm gì?" });
    expect(heading).toHaveFocus();
    expect(sideList()).toBeNull();
    expect(screen.getByText("Không giới hạn thời gian")).toBeInTheDocument();
    expect(screen.queryByRole("timer")).toBeNull();
    expect(position("Câu 1/3")).toBeInTheDocument();
    expect(screen.getByText("0/3 đã trả lời")).toBeInTheDocument();
    expect(screen.getByText("Tự lưu khi chọn")).toBeInTheDocument();
    expect(screen.getByText("Câu 1/3 · Chọn một đáp án · 2 điểm")).toBeInTheDocument();
    expect(screen.getAllByRole("radio")).toHaveLength(2);
    expect(screen.queryByRole("heading", { name: "Ràng buộc nào chặn trùng giá trị?" })).toBeNull();
    expect(screen.queryByText(/Hết giờ, hệ thống tự nộp/)).toBeNull();
    expect(questionMap().getAllByRole("button")).toHaveLength(3);
    expect(questionMap().getByRole("button", { name: "Câu 1, chưa trả lời" })).toHaveAttribute("aria-current", "step");
    expect(screen.getByRole("button", { name: "Câu trước" })).toBeDisabled();
    expect(screen.queryByRole("button", { name: "Học liệu tiếp" })).toBeNull();
  });

  it("saves a pick at once and counts it as answered", async () => {
    const { user, quiz } = await renderQuiz(practicePath);
    await user.click(await screen.findByRole("button", { name: "Bắt đầu làm bài" }));

    await user.click(await screen.findByRole("radio", { name: "Định danh duy nhất mỗi dòng" }));

    expect(await screen.findByText(SAVED_AT)).toBeInTheDocument();
    expect(quiz.saves).toEqual([
      { attemptId: ids.practiceAttempt, questionId: q1.id, optionIds: [q1.options[0].id] },
    ]);
    expect(screen.getByText("1/3 đã trả lời")).toBeInTheDocument();
    expect(questionMap().getByRole("button", { name: "Câu 1, đã trả lời" })).toBeInTheDocument();
  });

  it("moves between questions with the step buttons and the map, announcing and focusing each", async () => {
    const { user } = await renderQuiz(practicePath);
    await user.click(await screen.findByRole("button", { name: "Bắt đầu làm bài" }));
    await screen.findByRole("heading", { level: 2, name: "Khóa chính dùng để làm gì?" });

    await user.click(screen.getByRole("button", { name: "Câu tiếp" }));
    expect(screen.getByRole("heading", { level: 2, name: "Ràng buộc nào chặn trùng giá trị?" })).toHaveFocus();
    expect(screen.getByText("Câu 2 trên 3")).toBeInTheDocument();
    expect(position("Câu 2/3")).toBeInTheDocument();
    expect(screen.getByText("Câu 2/3 · Chọn tất cả đáp án đúng · 3 điểm")).toBeInTheDocument();
    expect(screen.getAllByRole("checkbox")).toHaveLength(3);
    expect(window.sessionStorage.getItem(`quiz-position:${ids.practiceAttempt}`)).toBe("1");

    await user.click(questionMap().getByRole("button", { name: "Câu 3, chưa trả lời" }));
    expect(screen.getByRole("heading", { level: 2, name: "Khóa ngoại tham chiếu tới đâu?" })).toHaveFocus();
    expect(screen.getByText("Câu 3 trên 3")).toBeInTheDocument();
    expect(questionMap().getByRole("button", { name: "Câu 3, chưa trả lời" })).toHaveAttribute("aria-current", "step");
    expect(screen.queryByRole("button", { name: "Câu tiếp" })).toBeNull();
    expect(screen.getAllByRole("button", { name: "Nộp bài" })).toHaveLength(2);

    await user.click(screen.getByRole("button", { name: "Câu trước" }));
    expect(screen.getByRole("heading", { level: 2, name: "Ràng buộc nào chặn trùng giá trị?" })).toHaveFocus();
  });

  it("scrolls without animation when the student prefers reduced motion", async () => {
    vi.stubGlobal("matchMedia", (query: string) => ({
      matches: query === "(prefers-reduced-motion: reduce)",
      media: query,
      addEventListener: () => undefined,
      removeEventListener: () => undefined,
    }));
    const scroll = vi.spyOn(Element.prototype, "scrollIntoView");
    const { user } = await renderQuiz(practicePath, practiceOpen());

    await user.click(await screen.findByRole("button", { name: "Câu tiếp" }));

    expect(scroll).toHaveBeenLastCalledWith({ block: "start", behavior: "auto" });
  });

  it("scrolls smoothly otherwise", async () => {
    const scroll = vi.spyOn(Element.prototype, "scrollIntoView");
    const { user } = await renderQuiz(practicePath, practiceOpen());

    await user.click(await screen.findByRole("button", { name: "Câu tiếp" }));

    expect(scroll).toHaveBeenLastCalledWith({ block: "start", behavior: "smooth" });
  });

  it("resumes on the question the student was on in this tab", async () => {
    window.sessionStorage.setItem(`quiz-position:${ids.practiceAttempt}`, "2");
    await renderQuiz(practicePath, practiceOpen());

    expect(await screen.findByRole("heading", { level: 2, name: "Khóa ngoại tham chiếu tới đâu?" })).toBeInTheDocument();
    expect(screen.getByText("3/3 đã trả lời")).toBeInTheDocument();
    expect(screen.getByText(SAVED_AT)).toBeInTheDocument();
    expect(screen.getByRole("radio", { name: "Khóa của bảng khác" })).not.toBeChecked();
    expect(sideList()).toBeNull();
  });

  it("resumes on the first unanswered question in a new tab", async () => {
    const partly = {
      ...fixtures.openPractice,
      questions: fixtures.openPractice.questions.map((q, i) => (i === 0 ? q : { ...q, selectedOptionIds: [] })),
    };
    await renderQuiz(practicePath, practiceOpen(partly));

    expect(await screen.findByRole("heading", { level: 2, name: "Ràng buộc nào chặn trùng giá trị?" })).toBeInTheDocument();
    expect(screen.getByText("1/3 đã trả lời")).toBeInTheDocument();
  });

  it("tells the student when the server hands back the attempt already open", async () => {
    // The start screen was read before another tab opened the attempt.
    const quiz = practiceOpen();
    quiz.overviews[ids.quizPractice] = fixtures.practiceOverview;
    const { user } = await renderQuiz(practicePath, quiz, {
      overrides: [
        http.post(startUrl, () => {
          quiz.overviews[ids.quizPractice] = fixtures.practiceOverviewOpen;
          return HttpResponse.json({ resumed: true, attempt: fixtures.openPractice });
        }),
      ],
    });

    await user.click(await screen.findByRole("button", { name: "Bắt đầu làm bài" }));

    expect(await screen.findByText("Bạn đang có một lượt chưa nộp: tiếp tục lượt đó.")).toBeInTheDocument();
    expect(await screen.findByText("3/3 đã trả lời")).toBeInTheDocument();
  });

  it("sends only the latest pick while a save of the same question is in flight", async () => {
    const hold = gate();
    const bodies: string[][] = [];
    const { user } = await renderQuiz(practicePath, quizState(), {
      overrides: [
        http.put(answerUrl, async ({ request }) => {
          bodies.push(((await request.json()) as { optionIds: string[] }).optionIds);
          await hold.wait();
          return HttpResponse.json({ savedAt: new Date().toISOString() });
        }),
      ],
    });
    await user.click(await screen.findByRole("button", { name: "Bắt đầu làm bài" }));
    await user.click(await screen.findByRole("button", { name: "Câu tiếp" }));

    await user.click(screen.getByRole("checkbox", { name: "PRIMARY KEY" }));
    expect(await screen.findByText("Đang lưu…")).toBeInTheDocument();
    await user.click(screen.getByRole("checkbox", { name: "CHECK" }));
    await user.click(screen.getByRole("checkbox", { name: "UNIQUE" }));
    await user.click(screen.getByRole("checkbox", { name: "CHECK" }));
    hold.open();

    expect(await screen.findByText(SAVED_AT)).toBeInTheDocument();
    const [primaryKey, , unique] = q2.options.map((o) => o.id);
    expect(bodies).toEqual([[primaryKey], [primaryKey, unique]]);
  });

  it("keeps a pick that failed to save on screen and saves it again on retry", async () => {
    const { user, quiz } = await renderQuiz(practicePath, quizState(), {
      overrides: [
        http.put(answerUrl, () => apiError(500, "INTERNAL", "Lỗi máy chủ, vui lòng thử lại sau."), { once: true }),
      ],
    });
    await user.click(await screen.findByRole("button", { name: "Bắt đầu làm bài" }));

    await user.click(await screen.findByRole("radio", { name: "Định danh duy nhất mỗi dòng" }));

    const alert = await screen.findByRole("alert");
    expect(alert).toHaveTextContent("Câu 1 chưa lưu được: Lỗi máy chủ, vui lòng thử lại sau.");
    expect(screen.getByText("Có câu chưa lưu được")).toBeInTheDocument();
    expect(screen.getByRole("radio", { name: "Định danh duy nhất mỗi dòng" })).toBeChecked();
    expect(quiz.saves).toEqual([]);

    await user.click(within(alert).getByRole("button", { name: "Thử lại" }));

    expect(await screen.findByText(SAVED_AT)).toBeInTheDocument();
    expect(screen.queryByRole("alert")).toBeNull();
    expect(quiz.saves).toEqual([
      { attemptId: ids.practiceAttempt, questionId: q1.id, optionIds: [q1.options[0].id] },
    ]);
  });

  it("opens the result when a save finds the attempt already closed by the server", async () => {
    const quiz = assessmentOpen();
    const { user, router } = await renderQuiz(assessmentPath, quiz, {
      overrides: [
        http.put(answerUrl, () => {
          closeAttempt(quiz, timedOut);
          return conflict("Đã hết giờ làm bài; câu trả lời không được lưu.");
        }),
      ],
    });

    await user.click(await screen.findByRole("checkbox", { name: "WHERE a = 1" }));

    expect(await screen.findByText("Lượt này được tự nộp khi hết giờ. Chỉ các câu đã lưu trước hạn được chấm.")).toBeInTheDocument();
    expect(screen.getByText("Đã hết giờ làm bài; câu trả lời không được lưu.")).toBeInTheDocument();
    expect(router.state.location.search).toBe(`?result=${ids.assessmentAttempt}`);
  });
});

describe("Quiz lesson: submitting", () => {
  it("asks before submitting with the number of blank questions, then shows the practice key", async () => {
    const { user, quiz, router } = await renderQuiz(practicePath);
    await user.click(await screen.findByRole("button", { name: "Bắt đầu làm bài" }));
    await user.click(await screen.findByRole("radio", { name: "Định danh duy nhất mỗi dòng" }));
    await user.click(screen.getByRole("button", { name: "Câu tiếp" }));
    await user.click(screen.getByRole("checkbox", { name: "PRIMARY KEY" }));

    await user.click(screen.getAllByRole("button", { name: "Nộp bài" })[0]);

    const dialog = await screen.findByRole("dialog", { name: "Nộp bài?" });
    expect(dialog).toHaveTextContent(
      "Bạn còn 1 câu chưa trả lời; các câu này tính 0 điểm. Sau khi nộp không sửa được nữa.",
    );
    await user.click(within(dialog).getByRole("button", { name: "Nộp bài" }));

    expect(await screen.findByText("Đã nộp bài.")).toBeInTheDocument();
    expect(router.state.location.search).toBe(`?result=${ids.practiceAttempt}`);
    expect(quiz.submits).toEqual([ids.practiceAttempt]);
    // Pending saves land before the submit.
    expect(quiz.saves.map((s) => s.questionId)).toEqual([q1.id, q2.id]);

    expect(await screen.findByRole("heading", { level: 2, name: /^Lượt 1 · nộp / })).toHaveFocus();
    expect(screen.getByText("5/6")).toBeInTheDocument();
    expect(screen.getByText("Quiz học liệu")).toBeInTheDocument();
    expect(screen.getByText("Đúng · 2 điểm")).toBeInTheDocument();
    expect(screen.getByText("Đúng · 3 điểm")).toBeInTheDocument();
    expect(screen.getByText("Sai · 0 điểm")).toBeInTheDocument();
    expect(screen.getAllByText("(đáp án đúng)", { exact: false })).toHaveLength(4);
    expect(screen.getAllByText("Bạn chọn")).toHaveLength(4);
    expect(screen.getByText("Giải thích: Khóa chính định danh duy nhất mỗi dòng.")).toBeInTheDocument();
    expect(screen.getByText("Khóa ngoại trỏ tới khóa của bảng khác.")).toBeInTheDocument();
    expect(screen.getByRole("heading", { name: "Nhận xét của giảng viên · Lê Thu Hương" })).toBeInTheDocument();
    expect(screen.getByText("Câu khóa ngoại cần xem lại.")).toBeInTheDocument();
    expect(sideList()).toBeInTheDocument();
  });

  it("confirms that every question is answered", async () => {
    const { user } = await renderQuiz(practicePath, practiceOpen());

    await user.click((await screen.findAllByRole("button", { name: "Nộp bài" }))[0]);

    expect(await screen.findByRole("dialog", { name: "Nộp bài?" })).toHaveTextContent(
      "Bạn đã trả lời tất cả câu hỏi. Sau khi nộp không sửa được nữa.",
    );
  });

  it("opens the result with the server's message when the attempt was closed before the submit", async () => {
    const quiz = practiceOpen();
    const { user, router } = await renderQuiz(practicePath, quiz, {
      overrides: [
        http.post(submitUrl, () => {
          closeAttempt(quiz, fixtures.resultPractice);
          return conflict("Lượt làm bài này đã được nộp.");
        }),
      ],
    });

    await user.click((await screen.findAllByRole("button", { name: "Nộp bài" }))[0]);
    await user.click(within(await screen.findByRole("dialog")).getByRole("button", { name: "Nộp bài" }));

    expect(await screen.findByText("Lượt làm bài này đã được nộp.")).toBeInTheDocument();
    expect(await screen.findByText("Đúng · 2 điểm")).toBeInTheDocument();
    expect(router.state.location.search).toBe(`?result=${ids.practiceAttempt}`);
  });

  it("shows an assessment result without any answer key", async () => {
    const quiz = quizState();
    const { user } = await renderQuiz(assessmentPath, quiz);
    await user.click(await screen.findByRole("button", { name: "Bắt đầu làm bài" }));
    expect(
      await screen.findByText("Lượt 1/2. Hết giờ, hệ thống tự nộp với các câu đã lưu. Rời trang không dừng đồng hồ."),
    ).toBeInTheDocument();
    expect(screen.getByRole("timer", { name: "Thời gian còn lại" })).toHaveTextContent(/^(10:00|09:5\d)$/);

    await user.click(screen.getAllByRole("button", { name: "Nộp bài" })[0]);
    const dialog = await screen.findByRole("dialog", { name: "Nộp bài?" });
    expect(dialog).toHaveTextContent("Bạn còn 3 câu chưa trả lời");
    await user.click(within(dialog).getByRole("button", { name: "Nộp bài" }));

    expect(await screen.findByRole("heading", { level: 2, name: /^Lượt 1 · nộp / })).toBeInTheDocument();
    expect(screen.getByText("7/10")).toBeInTheDocument();
    expect(screen.getByText("Đạt")).toBeInTheDocument();
    expect(screen.getByText("Chờ giảng viên xác nhận.")).toBeInTheDocument();
    expect(screen.getByText("Kết quả tính vào tiến độ là lượt tốt nhất: 70%. Còn 1 lượt.")).toBeInTheDocument();
    expect(screen.getByText("Quiz bài tập chỉ hiện điểm và kết quả Đạt, không hiện đáp án.")).toBeInTheDocument();
    expect(screen.queryByText(/đáp án đúng/)).toBeNull();
    expect(screen.queryByText("INSERT")).toBeNull();
    expect(screen.queryByText(/^(Đúng|Sai) · /)).toBeNull();
  });
});

describe("Quiz lesson: the clock", () => {
  it("warns at 5 and 1 minutes and submits once when time runs out", async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true, toFake: ["setInterval", "clearInterval", "Date"] });
    const quiz = assessmentOpen(5 + 2 / 60);
    await renderQuiz(assessmentPath, quiz);
    const clock = await screen.findByRole("timer", { name: "Thời gian còn lại" });
    expect(clock).toHaveTextContent(/^05:0[12]$/);
    expect(screen.queryByText(/^Còn \d phút/)).toBeNull();

    act(() => {
      vi.advanceTimersByTime(3_000);
    });
    expect(await screen.findByText("Còn 5 phút. Câu trả lời đã chọn được lưu tự động.")).toBeInTheDocument();

    act(() => {
      vi.advanceTimersByTime(4 * 60_000);
    });
    expect(await screen.findByText("Còn 1 phút. Câu trả lời đã chọn được lưu tự động.")).toBeInTheDocument();
    expect(quiz.submits).toEqual([]);

    act(() => {
      vi.advanceTimersByTime(61_000);
    });
    expect(await screen.findByText(TIME_UP, { selector: TOAST })).toBeInTheDocument();
    expect(await screen.findByText("Chờ giảng viên xác nhận.")).toBeInTheDocument();
    expect(quiz.submits).toEqual([ids.assessmentAttempt]);
  });

  it("only opens the result when the server closed the attempt at the deadline first", async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true, toFake: ["setInterval", "clearInterval", "Date"] });
    const quiz = assessmentOpen(2 / 60);
    let submits = 0;
    const { router } = await renderQuiz(assessmentPath, quiz, {
      overrides: [
        http.post(submitUrl, () => {
          submits += 1;
          closeAttempt(quiz, timedOut);
          return conflict("Đã hết giờ làm bài; hệ thống đã tự nộp với các câu đã lưu.");
        }),
      ],
    });
    // Under a minute left: only the latest warning shows.
    expect(await screen.findByText("Còn 1 phút. Câu trả lời đã chọn được lưu tự động.")).toBeInTheDocument();
    expect(screen.queryByText("Còn 5 phút. Câu trả lời đã chọn được lưu tự động.")).toBeNull();

    act(() => {
      vi.advanceTimersByTime(3_000);
    });

    expect(await screen.findByText(TIME_UP, { selector: TOAST })).toBeInTheDocument();
    expect(await screen.findByText("Lượt này được tự nộp khi hết giờ. Chỉ các câu đã lưu trước hạn được chấm.")).toBeInTheDocument();
    expect(screen.queryByText("Đã hết giờ làm bài; hệ thống đã tự nộp với các câu đã lưu.")).toBeNull();
    expect(router.state.location.search).toBe(`?result=${ids.assessmentAttempt}`);
    expect(submits).toBe(1);
  });

  it("goes straight to the result when the open attempt was closed while the student was away", async () => {
    // The start screen still names the attempt open; reading the attempt past its deadline closes it.
    const quiz = assessmentOpen();
    const { router } = await renderQuiz(assessmentPath, quiz, {
      overrides: [
        http.get(api("/me/attempts/:attemptId"), () => {
          closeAttempt(quiz, timedOut);
          return HttpResponse.json(timedOut);
        }),
      ],
    });

    expect(await screen.findByText(TIME_UP, { selector: TOAST })).toBeInTheDocument();
    expect(
      await screen.findByText("Lượt này được tự nộp khi hết giờ. Chỉ các câu đã lưu trước hạn được chấm."),
    ).toBeInTheDocument();
    expect(router.state.location.search).toBe(`?result=${ids.assessmentAttempt}`);
    expect(router.state.historyAction).toBe("REPLACE");
    expect(quiz.submits).toEqual([]);
    expect(screen.queryByRole("navigation", { name: "Chuyển nhanh đến câu hỏi" })).toBeNull();
  });
});

describe("Quiz lesson: result page", () => {
  it("shows the teacher's confirmation with notes named by question position, and no retry once passed", async () => {
    const quiz = quizState({
      overviews: { [ids.quizAssessment]: fixtures.assessmentOverviewPassed },
      attempts: { [ids.assessmentAttempt]: fixtures.resultAssessmentConfirmed },
    });
    const { user, router } = await renderQuiz(assessmentPath, quiz);

    await user.click(await screen.findByRole("link", { name: "Lượt 1" }));

    expect(
      await screen.findByRole("heading", { name: "Giảng viên đã xác nhận · Lê Thu Hương" }),
    ).toBeInTheDocument();
    expect(screen.getByText("Ổn, xem lại chi phí ghi của chỉ mục.")).toBeInTheDocument();
    expect(screen.getByText("Câu 2.")).toBeInTheDocument();
    expect(screen.getByText("Chỉ mục làm chậm thao tác nào?")).toBeInTheDocument();
    expect(screen.getByText("INSERT phải cập nhật mọi chỉ mục.")).toBeInTheDocument();
    expect(screen.getByText(/ · đã cập nhật$/)).toBeInTheDocument();
    expect(screen.queryByText("Chờ giảng viên xác nhận.")).toBeNull();
    expect(screen.queryByRole("button", { name: "Làm lại" })).toBeNull();

    await user.click(screen.getByRole("link", { name: "Về trang quiz" }));
    expect(await screen.findByRole("button", { name: "Làm lượt mới" })).toBeInTheDocument();
    expect(router.state.location.search).toBe("");
  });

  it("names the pass mark of a failed assessment", async () => {
    const failed = { ...fixtures.resultAssessmentPending, score: 4, percent: 40, passed: false };
    const quiz = quizState({
      overviews: { [ids.quizAssessment]: fixtures.assessmentOverviewOut },
      attempts: { [ids.assessmentAttempt]: failed },
    });
    await renderQuiz(`${assessmentPath}?result=${ids.assessmentAttempt}`, quiz);

    expect(await screen.findByText("Chưa đạt · cần 70%")).toBeInTheDocument();
    const retry = screen.getByRole("button", { name: "Làm lại" });
    expect(retry).toHaveAttribute("aria-disabled", "true");
    expect(retry).toHaveAttribute("title", "Bạn đã dùng hết 2 lượt");
  });

  it("starts a new practice attempt from the result", async () => {
    const next = { ...fixtures.openPracticeNew, id: "0199ffff-0000-7000-8000-000000000002", no: 2 };
    const quiz = quizState({
      overviews: { [ids.quizPractice]: fixtures.practiceOverviewDone },
      attempts: { [ids.practiceAttempt]: fixtures.resultPractice },
      starts: { [ids.quizPractice]: next },
    });
    const { user, router } = await renderQuiz(`${practicePath}?result=${ids.practiceAttempt}`, quiz);

    await user.click(await screen.findByRole("button", { name: "Làm lại" }));

    expect(await screen.findByRole("heading", { level: 2, name: "Khóa chính dùng để làm gì?" })).toBeInTheDocument();
    expect(router.state.location.search).toBe("");
    expect(sideList()).toBeNull();
  });

  it("leaves a result link that names an attempt still open for the attempt itself", async () => {
    const { router } = await renderQuiz(`${practicePath}?result=${ids.practiceAttempt}`, practiceOpen());

    expect(await screen.findByRole("heading", { level: 2, name: "Khóa chính dùng để làm gì?" })).toBeInTheDocument();
    expect(router.state.location.search).toBe("");
    expect(router.state.historyAction).toBe("REPLACE");
  });

  it("lists a practice attempt in the history and opens its result", async () => {
    const quiz = quizState({
      overviews: { [ids.quizPractice]: fixtures.practiceOverviewDone },
      attempts: { [ids.practiceAttempt]: fixtures.resultPractice },
    });
    const { user } = await renderQuiz(practicePath, quiz);

    const row = within(await screen.findByRole("region", { name: "Các lượt đã nộp" })).getByRole("listitem");
    expect(within(row).getByText("5/6 · 83%")).toBeInTheDocument();
    expect(within(row).queryByText(/Đạt/)).toBeNull();
    expect(screen.getByRole("button", { name: "Làm lượt mới" })).toBeInTheDocument();

    await user.click(within(row).getByRole("link", { name: "Lượt 1" }));
    expect(await screen.findByText("Sai · 0 điểm")).toBeInTheDocument();
    expect(screen.getByText(q3.options[1].body)).toBeInTheDocument();
  });
});
