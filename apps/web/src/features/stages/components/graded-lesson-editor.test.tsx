import { fireEvent, screen, waitFor, within } from "@testing-library/react";
import { beforeAll, describe, expect, it, vi } from "vitest";
import { authoringWorld, DOCUMENT_MEDIA_ID, type StageWorld } from "../api/msw-handlers";
import { DOCUMENT_TYPE_MESSAGE } from "../model/attachment-file";
import { CRITERION_POINTS_RANGE, CRITERION_DESCRIPTION_REQUIRED } from "../model/criterion-form";
import { mediaContentUrl } from "../model/image-file";
import { lessonEditorPath } from "../model/lesson-form";
import { QUESTION_NEEDS_CORRECT, QUESTION_TOO_FEW_OPTIONS, SINGLE_HAS_ONE_CORRECT } from "../model/question-form";
import type { Lesson, Question, StageVersion } from "../model/schemas";
import { fixtures, ids } from "../test/golden";
import { recordRequests, renderStages, stageUrl } from "../test/render-stages";

// The brief is written in a textarea standing in for the WYSIWYG editor, with the same props and handle.
vi.mock("./markdown-editor/markdown-editor", () => import("../test/markdown-editor-double"));

// The routes are lazy: load them once up front so the first test's timeouts do not include the import.
beforeAll(async () => {
  await import("../pages/lesson-editor-page");
  await import("../pages/stage-detail-page");
});

const quiz = fixtures.lessonQuiz;
const homework = fixtures.lessonHomework;
const [joinQuestion] = quiz.quiz?.questions ?? [];
const [starter] = homework.homework?.attachments ?? [];
const [criterion] = homework.homework?.criteria ?? [];
const lessonPath = (lessonId: string) => `/stage-versions/${ids.v2}/lessons/${lessonId}`;

/** The authoring draft with `change` applied to one lesson, and optionally another status. */
function worldWith(lessonId: string, change: (lesson: Lesson) => Lesson, status?: StageVersion["status"]): StageWorld {
  const world = authoringWorld();
  const draft = fixtures.stageVersionDraftAuthoring;
  const version = {
    ...draft,
    status: status ?? draft.status,
    lessons: draft.lessons.map((l) => (l.id === lessonId ? change(l) : l)),
  };
  return { ...world, versions: [fixtures.stageVersion, version] };
}

function withQuestions(questions: Question[]) {
  return worldWith(quiz.id, (l) => (l.quiz ? { ...l, quiz: { ...l.quiz, questions } } : l));
}

function card(name: RegExp): HTMLElement {
  const el = screen.getByRole("heading", { name }).closest<HTMLElement>('[data-slot="card"]');
  if (!el) throw new Error(`no card for ${String(name)}`);
  return el;
}

/** "Thiết lập" facts as label → value. */
function facts(): Record<string, string> {
  const settings = card(/^Thiết lập$/);
  const terms = within(settings).getAllByRole("term");
  return Object.fromEntries(terms.map((dt) => [dt.textContent, dt.nextElementSibling?.textContent ?? ""]));
}

function openQuiz(world: StageWorld = authoringWorld()) {
  return renderStages(lessonEditorPath(ids.stage, ids.v2, quiz.id), { world });
}

function openHomework(world: StageWorld = authoringWorld()) {
  return renderStages(lessonEditorPath(ids.stage, ids.v2, homework.id), { world });
}

const secondQuestion: Question = {
  id: "0199ffff-0000-7000-8000-0000000000b1",
  questionKey: "inner-join",
  type: "multi",
  bodySource: "INNER JOIN bỏ dòng nào?",
  explanationSource: "",
  points: 2,
  options: [
    { id: "0199ffff-0000-7000-8000-0000000000b2", body: "Dòng không khớp", isCorrect: true },
    { id: "0199ffff-0000-7000-8000-0000000000b3", body: "Dòng khớp", isCorrect: false },
  ],
};

describe("GradedLessonEditor: quiz", () => {
  it("shows the settings and every question with its correct answers marked", async () => {
    openQuiz();

    expect(await screen.findByRole("heading", { level: 1, name: quiz.title })).toBeInTheDocument();
    expect(document.title).toContain(quiz.title);
    expect(screen.getByText("Quiz bài tập · 1 câu · 1 điểm · đạt từ 70% · 2 lượt · 10 phút")).toBeInTheDocument();
    expect(facts()).toEqual({
      "Loại quiz": "Quiz bài tập (tính vào tiến độ)",
      "Ngưỡng đạt": "70%",
      "Số lượt": "2 lượt",
      "Thời gian": "10 phút",
      "Xáo trộn": "Câu hỏi và phương án",
      "Cách chấm": "Đúng toàn bộ phương án mới được điểm câu",
      "Giảng viên": "Xác nhận từng lượt nộp trong hàng chờ chấm",
    });
    expect(
      screen.getByText(
        "Sau khi nộp, học viên chỉ thấy điểm và kết quả Đạt hoặc Chưa đạt, không thấy đáp án. Lượt tốt nhất được tính.",
      ),
    ).toBeInTheDocument();

    const questions = card(/^Câu hỏi/);
    expect(within(questions).getByRole("heading", { name: "Câu hỏi 1 câu · 1 điểm" })).toBeInTheDocument();
    expect(questions).toHaveTextContent("LEFT JOIN giữ dòng nào?");
    expect(questions).toHaveTextContent("Một đáp án1 điểmkey join-left");
    const options = within(questions)
      .getAllByRole("listitem")
      .filter((li) => li.closest("ul") !== li.closest("ol"))
      .map((li) => li.textContent);
    expect(options).toContain("Đáp án đúng: Mọi dòng bảng trái");
    expect(options).toContain("Chỉ dòng khớp");
    expect(questions).toHaveTextContent("Giải thích: Giữ mọi dòng bảng trái.");
    expect(screen.queryByText("Chưa đủ để phát hành:")).toBeNull();
  });

  it("checks the options the way the server does before adding a question", async () => {
    const requests = recordRequests();
    const { user } = openQuiz();
    await user.click(await screen.findByRole("button", { name: "Thêm câu hỏi" }));
    const dialog = screen.getByRole("dialog", { name: "Thêm câu hỏi" });
    const options = within(dialog).getByRole("group", { name: /^Phương án/ });
    const submit = within(dialog).getByRole("button", { name: "Thêm câu hỏi" });

    await user.type(within(dialog).getByLabelText(/^Nội dung câu hỏi/), "COUNT(*) đếm gì?");
    await user.type(within(dialog).getByRole("textbox", { name: "Phương án 1" }), "Mọi dòng");
    await user.click(submit);
    expect(options).toHaveAccessibleDescription(QUESTION_TOO_FEW_OPTIONS);

    await user.type(within(dialog).getByRole("textbox", { name: "Phương án 2" }), "Dòng không NULL");
    await user.click(submit);
    expect(options).toHaveAccessibleDescription(QUESTION_NEEDS_CORRECT);

    await user.click(within(dialog).getByRole("checkbox", { name: "Phương án 1 là đáp án đúng" }));
    await user.click(within(dialog).getByRole("checkbox", { name: "Phương án 2 là đáp án đúng" }));
    await user.click(submit);
    expect(options).toHaveAccessibleDescription(SINGLE_HAS_ONE_CORRECT);
    expect(requests.of("POST", `${lessonPath(quiz.id)}/questions`)).toHaveLength(0);

    await user.click(within(dialog).getByRole("combobox", { name: "Loại câu" }));
    await user.click(await screen.findByRole("option", { name: "Nhiều đáp án" }));
    const points = within(dialog).getByRole("spinbutton", { name: /^Điểm/ });
    await user.clear(points);
    await user.type(points, "2");
    await user.type(within(dialog).getByLabelText(/^Giải thích/), "COUNT(*) đếm cả dòng có NULL.");
    await user.click(submit);

    expect(await screen.findByText("Đã thêm câu hỏi.")).toBeInTheDocument();
    const [post] = requests.of("POST", `${lessonPath(quiz.id)}/questions`);
    expect(JSON.parse(post.body)).toEqual({
      type: "multi",
      bodySource: "COUNT(*) đếm gì?",
      explanationSource: "COUNT(*) đếm cả dòng có NULL.",
      points: 2,
      options: [
        { body: "Mọi dòng", correct: true },
        { body: "Dòng không NULL", correct: true },
      ],
    });
    expect(screen.queryByRole("dialog", { name: "Thêm câu hỏi" })).toBeNull();
    expect(screen.getByRole("heading", { name: "Câu hỏi 2 câu · 3 điểm" })).toBeInTheDocument();
  });

  it("names the field a question still needs", async () => {
    const { user } = openQuiz();
    await user.click(await screen.findByRole("button", { name: "Thêm câu hỏi" }));
    const dialog = screen.getByRole("dialog", { name: "Thêm câu hỏi" });
    const points = within(dialog).getByRole("spinbutton", { name: /^Điểm/ });
    await user.clear(points);
    await user.type(points, "0");
    await user.click(within(dialog).getByRole("button", { name: "Thêm câu hỏi" }));

    expect(within(dialog).getByLabelText(/^Nội dung câu hỏi/)).toHaveAccessibleDescription(/Nhập nội dung câu hỏi\./);
    expect(points).toHaveAccessibleDescription("Điểm của câu hỏi là số nguyên từ 1 đến 100.");
    expect(points).toBeInvalid();
  });

  it("edits a question, keeping the ids of the options it keeps", async () => {
    const requests = recordRequests();
    const { user } = openQuiz();
    await user.click(await screen.findByRole("button", { name: "Sửa câu 1" }));
    const dialog = screen.getByRole("dialog", { name: "Sửa câu hỏi" });
    expect(within(dialog).getByLabelText(/^Nội dung câu hỏi/)).toHaveValue(joinQuestion.bodySource);
    expect(within(dialog).getByRole("checkbox", { name: "Phương án 1 là đáp án đúng" })).toBeChecked();
    const second = within(dialog).getByRole("textbox", { name: "Phương án 2" });
    await user.clear(second);
    await user.type(second, "Chỉ dòng khớp ở cả hai bảng");
    await user.type(within(dialog).getByRole("textbox", { name: "Phương án 3" }), "Không dòng nào");
    await user.click(within(dialog).getByRole("button", { name: "Lưu" }));

    expect(await screen.findByText("Đã lưu câu hỏi.")).toBeInTheDocument();
    const [patch] = requests.of("PATCH", `${lessonPath(quiz.id)}/questions/${joinQuestion.id}`);
    expect(JSON.parse(patch.body)).toMatchObject({
      options: [
        { id: joinQuestion.options[0].id, body: "Mọi dòng bảng trái", correct: true },
        { id: joinQuestion.options[1].id, body: "Chỉ dòng khớp ở cả hai bảng", correct: false },
        { body: "Không dòng nào", correct: false },
      ],
    });
    expect(card(/^Câu hỏi/)).toHaveTextContent("Không dòng nào");
  });

  it("previews the question through the server's renderer", async () => {
    const { user } = openQuiz();
    await user.click(await screen.findByRole("button", { name: "Sửa câu 1" }));
    const dialog = screen.getByRole("dialog", { name: "Sửa câu hỏi" });
    await user.click(within(dialog).getByRole("button", { name: "Xem trước" }));

    const preview = within(dialog).getByRole("region", { name: "Xem trước câu hỏi" });
    expect(await within(preview).findAllByRole("heading", { name: "Thiết kế bảng và khóa" })).toHaveLength(2);
    expect(within(dialog).getByRole("button", { name: "Cập nhật xem trước" })).toBeInTheDocument();
  });

  it("reorders and deletes questions", async () => {
    const requests = recordRequests();
    const { user } = openQuiz(withQuestions([joinQuestion, secondQuestion]));
    await screen.findByRole("heading", { name: "Câu hỏi 2 câu · 3 điểm" });
    const questions = card(/^Câu hỏi/);
    expect(within(questions).getByRole("button", { name: "Chuyển câu 1 lên" })).toBeDisabled();
    await user.click(within(questions).getByRole("button", { name: "Chuyển câu 2 lên" }));

    await waitFor(() => {
      expect(requests.of("PUT", `${lessonPath(quiz.id)}/questions/order`)).toHaveLength(1);
    });
    const [put] = requests.of("PUT", `${lessonPath(quiz.id)}/questions/order`);
    expect(JSON.parse(put.body)).toEqual({ questionIds: [secondQuestion.id, joinQuestion.id] });
    await waitFor(() => {
      expect(
        within(questions)
          .getAllByText(/JOIN (giữ|bỏ) dòng nào\?/)
          .map((el) => el.textContent),
      ).toEqual(["INNER JOIN bỏ dòng nào?", "LEFT JOIN giữ dòng nào?"]);
    });

    await user.click(within(questions).getByRole("button", { name: "Xóa câu 1" }));
    const confirm = screen.getByRole("dialog", { name: "Xóa câu hỏi?" });
    expect(confirm).toHaveTextContent("Câu hỏi bị xóa khỏi bản nháp này. Phiên bản đã phát hành không bị ảnh hưởng.");
    await user.click(within(confirm).getByRole("button", { name: "Xóa câu hỏi" }));

    expect(await screen.findByText("Đã xóa câu hỏi.")).toBeInTheDocument();
    expect(requests.of("DELETE", `${lessonPath(quiz.id)}/questions/${secondQuestion.id}`)).toHaveLength(1);
    expect(await screen.findByRole("heading", { name: "Câu hỏi 1 câu · 1 điểm" })).toBeInTheDocument();
  });

  it("lists what still blocks publishing a draft quiz", async () => {
    const unanswered = { ...joinQuestion, options: joinQuestion.options.map((o) => ({ ...o, isCorrect: false })) };
    openQuiz(withQuestions([unanswered]));

    expect(await screen.findByText("Chưa đủ để phát hành:")).toBeInTheDocument();
    expect(screen.getByText(`"${quiz.title}" có 1 câu chưa đánh dấu đáp án đúng.`)).toBeInTheDocument();
    expect(within(card(/^Câu hỏi/)).getByText("Chưa có đáp án đúng")).toBeInTheDocument();
  });

  it("starts an empty quiz from its first question", async () => {
    const { user } = openQuiz(withQuestions([]));

    expect(await screen.findByText(`"${quiz.title}" chưa có câu hỏi.`)).toBeInTheDocument();
    const questions = card(/^Câu hỏi/);
    expect(within(questions).getByText("Chưa có câu hỏi")).toBeInTheDocument();
    expect(within(questions).queryByRole("button", { name: "Thêm câu hỏi" })).toBeNull();
    await user.click(within(questions).getByRole("button", { name: "Thêm câu hỏi đầu tiên" }));
    expect(screen.getByRole("dialog", { name: "Thêm câu hỏi" })).toBeInTheDocument();
  });

  it("saves the quiz settings from the lesson dialog", async () => {
    const requests = recordRequests();
    const { user } = openQuiz();
    await user.click(await screen.findByRole("button", { name: "Sửa thiết lập" }));
    const dialog = screen.getByRole("dialog", { name: "Sửa học liệu" });
    const pass = within(dialog).getByLabelText(/^Ngưỡng đạt \(%\)/);
    await user.clear(pass);
    await user.type(pass, "80");
    await user.click(within(dialog).getByRole("button", { name: "Lưu" }));

    await waitFor(() => {
      expect(facts()["Ngưỡng đạt"]).toBe("80%");
    });
    const [patch] = requests.of("PATCH", lessonPath(quiz.id));
    expect(JSON.parse(patch.body)).toMatchObject({ quiz: { mode: "assessment", passPercent: 80 } });
    expect(card(/^Câu hỏi/)).toHaveTextContent("LEFT JOIN giữ dòng nào?");
  });

  it("goes back to the stage once the quiz became a reading", async () => {
    const { user, router } = openQuiz();
    await user.click(await screen.findByRole("button", { name: "Sửa thiết lập" }));
    const dialog = screen.getByRole("dialog", { name: "Sửa học liệu" });
    await user.click(within(dialog).getByRole("combobox", { name: /Loại/ }));
    await user.click(await screen.findByRole("option", { name: "Bài đọc (Markdown)" }));
    await user.type(within(dialog).getByLabelText(/^Nội dung Markdown/), "# JOIN");
    await user.click(within(dialog).getByRole("button", { name: "Lưu" }));
    const confirm = await screen.findByRole("dialog", { name: `Đổi loại "${quiz.title}"?` });
    await user.click(within(confirm).getByRole("button", { name: "Đổi loại" }));

    await waitFor(() => {
      expect(router.state.location.pathname).toBe(`/admin/stages/${ids.stage}`);
    });
  });

  it("offers a published quiz read-only", async () => {
    openQuiz(worldWith(quiz.id, (l) => l, "published"));

    expect(
      await screen.findByText(/v2 đã phát hành nên không sửa được\. Nhân bản chặng thành bản nháp mới để sửa/),
    ).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Về chặng" })).toHaveAttribute("href", stageUrl(ids.stage, ids.v2));
    for (const name of ["Sửa thiết lập", "Thêm câu hỏi", "Sửa câu 1", "Xóa câu 1", "Chuyển câu 1 xuống"]) {
      expect(screen.queryByRole("button", { name })).toBeNull();
    }
    expect(card(/^Câu hỏi/)).toHaveTextContent("LEFT JOIN giữ dòng nào?");
  });
});

describe("GradedLessonEditor: homework", () => {
  const brief = () => screen.getByRole("textbox", { name: "Đề bài (Markdown)" });

  it("shows the settings, brief, files and rubric", async () => {
    openHomework();

    expect(await screen.findByRole("heading", { level: 1, name: homework.title })).toBeInTheDocument();
    expect(facts()).toEqual({
      "Ngưỡng đạt": "70% tổng điểm (từ 42/60)",
      "Cách nộp": "Đường dẫn repo và/hoặc file",
      "Giới hạn file": "5 file, mỗi file ≤ 20 MB",
      "Định dạng": "zip, pdf, md, txt, sql, png, jpg, jpeg",
    });
    expect(screen.getByText("Hạn nộp đặt riêng cho từng lớp trong trang lớp, không đặt ở đây.")).toBeInTheDocument();
    expect(brief()).toHaveValue("# Viết truy vấn JOIN");
    expect(screen.getByRole("button", { name: "Lưu đề bài" })).toBeDisabled();

    const files = screen.getByRole("group", { name: "File đính kèm" });
    expect(within(files).getByRole("link", { name: /starter\.zip/ })).toHaveAttribute(
      "href",
      mediaContentUrl(starter.mediaId),
    );

    const rubric = card(/^Tiêu chí chấm/);
    expect(within(rubric).getByRole("heading", { name: "Tiêu chí chấm 1 tiêu chí · 60 điểm" })).toBeInTheDocument();
    const rows = within(rubric)
      .getAllByRole("row")
      .map((r) => r.textContent);
    expect(rows[1]).toContain("Truy vấn chạy đúng60");
    expect(within(rubric).getByRole("rowheader", { name: "Tổng" }).closest("tr")).toHaveTextContent("Tổng60");
  });

  it("saves the brief", async () => {
    const requests = recordRequests();
    const { user } = openHomework();
    await screen.findByRole("heading", { level: 1, name: homework.title });
    await user.clear(brief());
    await user.type(brief(), "# Viết truy vấn LEFT JOIN");
    const save = screen.getByRole("button", { name: "Lưu đề bài" });
    expect(save).toBeEnabled();
    await user.click(save);

    expect(await screen.findByText("Đã lưu đề bài.")).toBeInTheDocument();
    const [patch] = requests.of("PATCH", lessonPath(homework.id));
    expect(JSON.parse(patch.body)).toEqual({ homework: { briefSource: "# Viết truy vấn LEFT JOIN" } });
    await waitFor(() => {
      expect(screen.getByRole("button", { name: "Lưu đề bài" })).toBeDisabled();
    });
  });

  it("refuses a brief with a link the server would reject", async () => {
    const requests = recordRequests();
    const { user } = openHomework();
    await screen.findByRole("heading", { level: 1, name: homework.title });
    fireEvent.change(brief(), { target: { value: "[tải về](ftp://files.example/de-bai.zip)" } });
    await user.click(screen.getByRole("button", { name: "Lưu đề bài" }));

    const alert = (await screen.findByText(/Gỡ các ảnh\/liên kết không hợp lệ trước khi lưu/)).closest(
      '[role="alert"]',
    );
    expect(alert).toHaveTextContent("ftp://files.example/de-bai.zip");
    expect(requests.of("PATCH", lessonPath(homework.id))).toHaveLength(0);
  });

  it("lists what still blocks publishing a draft homework", async () => {
    openHomework(
      worldWith(homework.id, (l) =>
        l.homework ? { ...l, homework: { ...l.homework, briefSource: "", criteria: [], maxPoints: 0 } } : l,
      ),
    );

    expect(await screen.findByText(`"${homework.title}" chưa có đề bài.`)).toBeInTheDocument();
    expect(screen.getByText(`"${homework.title}" chưa có tiêu chí chấm.`)).toBeInTheDocument();
    expect(within(card(/^Tiêu chí chấm/)).getByText("Chưa có tiêu chí")).toBeInTheDocument();
  });

  it("adds a criterion after checking it the way the server does", async () => {
    const requests = recordRequests();
    const { user } = openHomework();
    await user.click(await screen.findByRole("button", { name: "Thêm tiêu chí" }));
    const dialog = screen.getByRole("dialog", { name: "Thêm tiêu chí chấm" });
    const description = within(dialog).getByRole("textbox", { name: /^Tiêu chí/ });
    const points = within(dialog).getByRole("spinbutton", { name: /^Điểm tối đa/ });
    await user.type(points, "0");
    await user.click(within(dialog).getByRole("button", { name: "Thêm tiêu chí" }));

    expect(description).toHaveAccessibleDescription(CRITERION_DESCRIPTION_REQUIRED);
    expect(points).toHaveAccessibleDescription(new RegExp(CRITERION_POINTS_RANGE.replace(/\./g, "\\.")));
    expect(requests.of("POST", `${lessonPath(homework.id)}/criteria`)).toHaveLength(0);

    await user.type(description, "Đặt tên cột rõ ràng");
    await user.clear(points);
    await user.type(points, "40");
    await user.click(within(dialog).getByRole("button", { name: "Thêm tiêu chí" }));

    expect(await screen.findByText("Đã thêm tiêu chí.")).toBeInTheDocument();
    const [post] = requests.of("POST", `${lessonPath(homework.id)}/criteria`);
    expect(JSON.parse(post.body)).toEqual({ description: "Đặt tên cột rõ ràng", maxPoints: 40 });
    const rubric = card(/^Tiêu chí chấm/);
    expect(within(rubric).getByRole("rowheader", { name: "Tổng" }).closest("tr")).toHaveTextContent("Tổng100");
    expect(facts()["Ngưỡng đạt"]).toBe("70% tổng điểm (từ 70/100)");
  });

  it("edits, reorders and deletes criteria", async () => {
    const second = { id: "0199ffff-0000-7000-8000-0000000000c1", description: "Giải thích cách làm", maxPoints: 40 };
    const requests = recordRequests();
    const { user } = openHomework(
      worldWith(homework.id, (l) =>
        l.homework ? { ...l, homework: { ...l.homework, criteria: [criterion, second], maxPoints: 100 } } : l,
      ),
    );
    await screen.findByRole("heading", { name: "Tiêu chí chấm 2 tiêu chí · 100 điểm" });
    const rubric = card(/^Tiêu chí chấm/);

    await user.click(within(rubric).getByRole("button", { name: "Chuyển tiêu chí 2 lên" }));
    await waitFor(() => {
      expect(requests.of("PUT", `${lessonPath(homework.id)}/criteria/order`)).toHaveLength(1);
    });
    const [put] = requests.of("PUT", `${lessonPath(homework.id)}/criteria/order`);
    expect(JSON.parse(put.body)).toEqual({ criterionIds: [second.id, criterion.id] });

    await user.click(within(rubric).getByRole("button", { name: "Sửa tiêu chí 2" }));
    const dialog = screen.getByRole("dialog", { name: "Sửa tiêu chí chấm" });
    expect(within(dialog).getByRole("textbox", { name: /^Tiêu chí/ })).toHaveValue(criterion.description);
    const points = within(dialog).getByRole("spinbutton", { name: /^Điểm tối đa/ });
    await user.clear(points);
    await user.type(points, "50");
    await user.click(within(dialog).getByRole("button", { name: "Lưu" }));
    expect(await screen.findByText("Đã lưu tiêu chí.")).toBeInTheDocument();
    const [patch] = requests.of("PATCH", `${lessonPath(homework.id)}/criteria/${criterion.id}`);
    expect(JSON.parse(patch.body)).toEqual({ description: criterion.description, maxPoints: 50 });

    await user.click(within(rubric).getByRole("button", { name: "Xóa tiêu chí 1" }));
    const confirm = screen.getByRole("dialog", { name: "Xóa tiêu chí?" });
    expect(confirm).toHaveTextContent("Tiêu chí bị xóa khỏi bản nháp này; tổng điểm bài tập giảm theo.");
    await user.click(within(confirm).getByRole("button", { name: "Xóa tiêu chí" }));
    expect(await screen.findByText("Đã xóa tiêu chí.")).toBeInTheDocument();
    expect(requests.of("DELETE", `${lessonPath(homework.id)}/criteria/${second.id}`)).toHaveLength(1);
    await waitFor(() => {
      expect(within(rubric).getByRole("rowheader", { name: "Tổng" }).closest("tr")).toHaveTextContent("Tổng50");
    });
  });

  it("attaches a pdf, sending the content type the upload ticket was signed with", async () => {
    const requests = recordRequests();
    const { user } = openHomework();
    await screen.findByRole("heading", { level: 1, name: homework.title });
    // The browser knows no type for some documents; the ticket's signed one is what storage expects.
    const file = new File(["%PDF-1.4"], "de-bai.pdf", { type: "" });
    await user.upload(screen.getByLabelText("Chọn file đính kèm"), file);

    expect(await screen.findByText("Đã đính kèm de-bai.pdf.")).toBeInTheDocument();
    const [init] = requests.of("POST", "/media/uploads");
    expect(JSON.parse(init.body)).toEqual({ kind: "document", fileName: "de-bai.pdf", contentType: "", sizeBytes: 8 });
    const put = requests.seen.find((r) => r.method === "PUT" && r.path.startsWith("/put/"));
    expect(put?.headers.get("content-type")).toBe("application/pdf");
    const [patch] = requests.of("PATCH", lessonPath(homework.id));
    expect(JSON.parse(patch.body)).toEqual({ homework: { attachmentMediaIds: [starter.mediaId, DOCUMENT_MEDIA_ID] } });
    const files = screen.getByRole("group", { name: "File đính kèm" });
    expect(await within(files).findByRole("link", { name: /de-bai\.pdf/ })).toHaveAttribute(
      "href",
      mediaContentUrl(DOCUMENT_MEDIA_ID),
    );
  });

  it("refuses a file type the server would reject before uploading it", async () => {
    const requests = recordRequests();
    openHomework();
    await screen.findByRole("heading", { level: 1, name: homework.title });
    const input = screen.getByLabelText("Chọn file đính kèm");
    fireEvent.change(input, { target: { files: [new File(["a,b"], "diem.csv", { type: "text/csv" })] } });

    expect(await screen.findByRole("alert")).toHaveTextContent(DOCUMENT_TYPE_MESSAGE);
    expect(screen.getByRole("button", { name: /Tải file lên/ })).toHaveAccessibleDescription(DOCUMENT_TYPE_MESSAGE);
    expect(requests.of("POST", "/media/uploads")).toHaveLength(0);
  });

  it("removes an attachment", async () => {
    const requests = recordRequests();
    const { user } = openHomework();
    await user.click(await screen.findByRole("button", { name: `Gỡ file ${starter.fileName}` }));

    expect(await screen.findByText(`Đã gỡ ${starter.fileName}.`)).toBeInTheDocument();
    const [patch] = requests.of("PATCH", lessonPath(homework.id));
    expect(JSON.parse(patch.body)).toEqual({ homework: { attachmentMediaIds: [] } });
    expect(await screen.findByText("Chưa có file kèm.")).toBeInTheDocument();
  });

  it("offers a published homework read-only", async () => {
    openHomework(
      worldWith(
        homework.id,
        (l) => (l.homework ? { ...l, homework: { ...l.homework, briefHtml: "<h1>Viết truy vấn JOIN</h1>" } } : l),
        "published",
      ),
    );

    expect(await screen.findByRole("heading", { level: 1, name: "Viết truy vấn JOIN" })).toBeInTheDocument();
    expect(screen.queryByRole("textbox", { name: "Đề bài (Markdown)" })).toBeNull();
    for (const name of ["Lưu đề bài", "Thêm tiêu chí", "Sửa tiêu chí 1", `Gỡ file ${starter.fileName}`]) {
      expect(screen.queryByRole("button", { name })).toBeNull();
    }
    expect(screen.queryByLabelText("Chọn file đính kèm")).toBeNull();
    expect(screen.queryByRole("columnheader", { name: "Thao tác" })).toBeNull();
    expect(screen.getByRole("link", { name: /starter\.zip/ })).toHaveAttribute(
      "href",
      mediaContentUrl(starter.mediaId),
    );
  });
});
