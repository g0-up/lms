import { screen, waitFor, within } from "@testing-library/react";
import { http, HttpResponse } from "msw";
import { describe, expect, it } from "vitest";
import { api, apiError, server } from "@/shared/test/msw";
import { renderRoutes } from "@/shared/test/render";
import { quizReviewHandlers } from "../api/msw-handlers";
import type { AttemptGrant, ClassQuizzes } from "../model/quiz-schemas";
import { fixtures } from "../test/golden";
import { polyfillRadix } from "../test/render-grading";
import { QuizTab } from "./quiz-tab";

polyfillRadix();

const quizzesUrl = api("/classes/:classId/quizzes");
const grantsUrl = api("/classes/:classId/members/:memberId/grants");
const classId = fixtures.classQuizzes.class.id;

function renderTab(data: ClassQuizzes = fixtures.classQuizzes, overrides: Parameters<typeof server.use> = [], base: "/teach" | "/admin" = "/teach") {
  // Each `use` takes precedence over the previous ones.
  server.use(...quizReviewHandlers);
  server.use(http.get(quizzesUrl, () => HttpResponse.json(data)));
  if (overrides.length > 0) server.use(...overrides);
  return renderRoutes([{ path: `${base}/classes/:classId`, element: <QuizTab classId={classId} basePath={base} /> }], {
    initialEntries: [`${base}/classes/${classId}`],
    withToaster: true,
  });
}

const card = (title: string) => screen.getByRole("region", { name: title });
const rowOf = (title: string, name: string) => {
  const row = within(card(title)).getByText(name).closest("tr");
  if (!row) throw new Error(`row of ${name}`);
  return within(row);
};
/** The counts line entry of a card whose own text is `label`, e.g. "chờ xác nhận" in "1 chờ xác nhận". */
const stat = (title: string, label: string) => within(card(title)).getByText(new RegExp(`^${label}$`));

const PRACTICE = "Luyện tập: Truy vấn SQL cơ bản";
const ASSESSMENT = "Kiểm tra cuối chặng Database";
const GO = "Kiểm tra cuối chặng Go";
const HTML = "Kiểm tra cuối chặng HTML CSS JS";
/** The rows of "Tỉ lệ trả lời đúng từng câu" in a card: one list item per question. */
const questionRows = (title: string) => {
  const heading = within(card(title)).getByRole("heading", { level: 3, name: "Tỉ lệ trả lời đúng từng câu" });
  const list = heading.parentElement?.querySelector("ul");
  if (!list) throw new Error(`question rates of ${title}`);
  return within(list).getAllByRole("listitem");
};
const grantButton = (title: string, name: string) =>
  rowOf(title, name).getByRole("button", { name: `Cấp thêm lượt cho ${name}` });
const reviewHref = (aid: string, base = "/teach") => `${base}/grading/quiz/${aid}?from=quiz`;

describe("Class quiz tab", () => {
  it("shows each assessment with its settings, counts and members' attempts", async () => {
    renderTab();

    expect(await screen.findByRole("heading", { level: 2, name: ASSESSMENT })).toBeInTheDocument();
    const assessment = within(card(ASSESSMENT));
    expect(assessment.getByText("Database · 2 lượt · 10 phút · giảng viên xác nhận từng lượt · đạt từ 70%")).toBeInTheDocument();
    expect(assessment.getByText("Quiz bài tập")).toBeInTheDocument();
    expect(stat(ASSESSMENT, "/7 đã làm")).toHaveTextContent("6/7 đã làm");
    expect(stat(ASSESSMENT, "đạt")).toHaveTextContent("5 đạt");
    expect(stat(ASSESSMENT, "chờ xác nhận")).toHaveTextContent("1 chờ xác nhận");
    expect(assessment.getByText(/^Điểm TB lượt tốt nhất/)).toHaveTextContent("Điểm TB lượt tốt nhất 77%");
    expect(stat(ASSESSMENT, "lượt đã nộp")).toHaveTextContent("7 lượt đã nộp");
    expect(assessment.getAllByRole("columnheader").map((th) => th.textContent)).toEqual([
      "Học viên",
      "Lượt đã dùng",
      "Tốt nhất",
      "Trạng thái",
      "Lượt gần nhất",
      "Thao tác",
    ]);

    const ha = rowOf(ASSESSMENT, "Hoàng Thu Hà");
    expect(ha.getByText("2/2")).toBeInTheDocument();
    expect(ha.getByText("80%")).toBeInTheDocument();
    expect(ha.getByText("Đạt 80%")).toBeInTheDocument();
    expect(ha.getByText(/^#2 · 80% · /)).toBeInTheDocument();
    expect(ha.getByText("Chờ xác nhận")).toBeInTheDocument();
    expect(ha.getByRole("link", { name: "Xác nhận lượt #2 của Hoàng Thu Hà" })).toHaveAttribute(
      "href",
      reviewHref("0199ffff-0000-7000-8000-000000000018"),
    );

    const khang = rowOf(ASSESSMENT, "Vũ Đức Khang");
    expect(khang.getByText("Chưa đạt 40%")).toBeInTheDocument();
    expect(khang.getByText("Đã xác nhận")).toBeInTheDocument();
    expect(khang.getByRole("link", { name: "Xem lượt #1 của Vũ Đức Khang" })).toHaveTextContent("Xem");

    const dung = rowOf(ASSESSMENT, "Phạm Minh Dũng");
    expect(dung.getByText("0/2")).toBeInTheDocument();
    expect(dung.getByText("Chưa làm")).toBeInTheDocument();
    expect(dung.queryByRole("link")).toBeNull();
  });

  it("marks a member who used every attempt without passing", async () => {
    renderTab();

    const go = "Kiểm tra cuối chặng Go";
    await screen.findByRole("heading", { level: 2, name: go });
    expect(within(card(go)).getByText(/^Golang basic · 2 lượt · 15 phút · /)).toBeInTheDocument();
    expect(rowOf(go, "Lê Văn Cường").getByText("Chưa đạt 60% · hết lượt")).toBeInTheDocument();
  });

  it("shows practice without attempt limit, threshold or pass counts, with a way to comment", async () => {
    renderTab();

    expect(await screen.findByRole("heading", { level: 2, name: PRACTICE })).toBeInTheDocument();
    const practice = within(card(PRACTICE));
    expect(practice.getByText("Database · không giới hạn lượt · nhận xét khi cần")).toBeInTheDocument();
    expect(practice.getByText("Quiz học liệu")).toBeInTheDocument();
    expect(practice.queryByText(/^chờ xác nhận$/)).toBeNull();
    expect(practice.queryByText(/^đạt$/)).toBeNull();

    const an = rowOf(PRACTICE, "Nguyễn Hoàng An");
    expect(an.getByText("1")).toBeInTheDocument();
    expect(an.getByText("Đã làm")).toBeInTheDocument();
    expect(an.getByText("Chưa nhận xét")).toBeInTheDocument();
    expect(an.getByRole("link", { name: "Xem và nhận xét lượt #1 của Nguyễn Hoàng An" })).toHaveTextContent(
      "Xem, nhận xét",
    );
    expect(an.getByRole("link", { name: /Nguyễn Hoàng An/ })).toHaveAttribute(
      "href",
      reviewHref("0199ffff-0000-7000-8000-000000000005"),
    );
    expect(rowOf(PRACTICE, "Hoàng Thu Hà").getByText("Chưa làm")).toBeInTheDocument();

    const goPractice = "Luyện tập: Goroutine và channel";
    expect(rowOf(goPractice, "Trần Thị Bích").getByText("Đang làm")).toBeInTheDocument();
    expect(rowOf(goPractice, "Lê Văn Cường").getByText("Đã nhận xét")).toBeInTheDocument();
  });

  it("shows a quiz nobody took yet without an average", async () => {
    renderTab();

    const html = "Kiểm tra cuối chặng HTML CSS JS";
    await screen.findByRole("heading", { level: 2, name: html });
    expect(within(card(html)).getByText(/^Điểm TB lượt tốt nhất/)).toHaveTextContent("Điểm TB lượt tốt nhất —");
    expect(within(card(html)).queryAllByRole("link")).toHaveLength(0);
  });

  it("keeps an admin's review links in the admin area", async () => {
    renderTab(fixtures.classQuizzes, [], "/admin");

    await screen.findByRole("heading", { level: 2, name: ASSESSMENT });
    expect(rowOf(ASSESSMENT, "Hoàng Thu Hà").getByRole("link")).toHaveAttribute(
      "href",
      reviewHref("0199ffff-0000-7000-8000-000000000018", "/admin"),
    );
  });

  it("says so when the course has no quiz", async () => {
    renderTab(fixtures.classQuizzesNone);

    expect(await screen.findByText("Khóa học không có quiz")).toBeInTheDocument();
    expect(screen.getByText("Phiên bản khóa học của lớp này không có học liệu loại quiz.")).toBeInTheDocument();
  });

  it("shows the API's refusal outside the grader's classes, without a retry", async () => {
    renderTab(fixtures.classQuizzes, [
      http.get(quizzesUrl, () => apiError(403, "FORBIDDEN", "Bạn chỉ xem được lớp mình phụ trách.")),
    ]);

    expect(await screen.findByRole("alert")).toHaveTextContent("Bạn chỉ xem được lớp mình phụ trách.");
    expect(screen.queryByRole("button", { name: "Thử lại" })).toBeNull();
  });

  it("offers a retry when the server cannot be reached", async () => {
    let calls = 0;
    const { user } = renderTab(fixtures.classQuizzes, [
      http.get(quizzesUrl, () => {
        calls += 1;
        return calls === 1 ? HttpResponse.error() : HttpResponse.json(fixtures.classQuizzes);
      }),
    ]);

    expect(await screen.findByRole("alert")).toHaveTextContent("Không kết nối được máy chủ. Thử lại sau.");
    await user.click(screen.getByRole("button", { name: "Thử lại" }));
    expect(await screen.findByRole("heading", { level: 2, name: ASSESSMENT })).toBeInTheDocument();
  });

  it("shows each question's correct rate and flags those under 30% in words", async () => {
    renderTab();

    await screen.findByRole("heading", { level: 2, name: ASSESSMENT });
    const database = questionRows(ASSESSMENT);
    expect(database.map((li) => within(li).getByRole("img").getAttribute("aria-label"))).toEqual([
      "100% trả lời đúng",
      "71% trả lời đúng",
      "86% trả lời đúng",
      "57% trả lời đúng",
      "43% trả lời đúng",
    ]);
    expect(database[0]).toHaveTextContent(/^Câu 1\. Khóa ngoại tham chiếu tới cột nào/);
    expect(within(card(ASSESSMENT)).queryByText("Dưới 30%: xem lại câu hỏi")).toBeNull();

    const go = questionRows(GO);
    expect(within(go[3]).getByRole("img", { name: "0% trả lời đúng" })).toBeInTheDocument();
    expect(within(go[3]).getByText("Dưới 30%: xem lại câu hỏi")).toBeInTheDocument();
    expect(within(go[2]).getByRole("img", { name: "33% trả lời đúng" })).toBeInTheDocument();
    expect(within(go[2]).queryByText("Dưới 30%: xem lại câu hỏi")).toBeNull();
    expect(within(card(GO)).getAllByText("Dưới 30%: xem lại câu hỏi")).toHaveLength(1);

    expect(within(card(HTML)).getByText("Chưa có lượt nộp nào.")).toBeInTheDocument();
    expect(within(card(HTML)).queryAllByRole("img")).toHaveLength(0);
  });

  it("offers extra attempts on an assessment to members who have not passed", async () => {
    renderTab();

    await screen.findByRole("heading", { level: 2, name: ASSESSMENT });
    expect(grantButton(ASSESSMENT, "Vũ Đức Khang")).toHaveTextContent("Cấp thêm lượt");
    expect(grantButton(ASSESSMENT, "Phạm Minh Dũng")).toBeInTheDocument();
    expect(rowOf(ASSESSMENT, "Hoàng Thu Hà").queryByRole("button")).toBeNull();
    expect(within(card(PRACTICE)).queryAllByRole("button")).toHaveLength(0);

    // Out of attempts: the grant is the way on, so it stands out from the plain one.
    expect(grantButton(GO, "Lê Văn Cường")).toHaveClass("border-navy-700");
    expect(grantButton(GO, "Nguyễn Hoàng An")).not.toHaveClass("border-navy-700");
    expect(within(card(ASSESSMENT)).queryByText("Lịch sử cấp thêm lượt")).toBeNull();
  });

  it("grants extra attempts with a reason and reloads the tab", async () => {
    const sent: unknown[] = [];
    let loads = 0;
    const { user } = renderTab(fixtures.classQuizzes, [
      http.get(quizzesUrl, () => {
        loads += 1;
        return HttpResponse.json(fixtures.classQuizzes);
      }),
      http.post(grantsUrl, async ({ request, params }) => {
        sent.push({ memberId: params.memberId, body: await request.json() });
        return HttpResponse.json(grant, { status: 201 });
      }),
    ]);

    await screen.findByRole("heading", { level: 2, name: GO });
    await user.click(grantButton(GO, "Lê Văn Cường"));
    const dialog = await screen.findByRole("dialog", { name: "Cấp thêm lượt cho Lê Văn Cường" });
    expect(within(dialog).getByText("Kiểm tra cuối chặng Go · đã dùng 2/2 lượt.")).toBeInTheDocument();
    expect(within(dialog).getByText("Lưu vào lịch sử cấp lượt của lớp.")).toBeInTheDocument();

    await user.click(within(dialog).getByRole("button", { name: "Cấp thêm lượt" }));
    expect(await within(dialog).findByText("Nhập lý do cấp thêm lượt. Lý do được lưu vào lịch sử.")).toBeInTheDocument();
    expect(sent).toHaveLength(0);

    await user.click(within(dialog).getByRole("combobox", { name: "Số lượt cấp thêm" }));
    await user.click(await screen.findByRole("option", { name: "2 lượt" }));
    await user.type(within(dialog).getByRole("textbox", { name: /^Lý do/ }), "  Mất kết nối khi đang làm bài ");
    await user.click(within(dialog).getByRole("button", { name: "Cấp thêm lượt" }));

    expect(await screen.findByText("Đã cấp thêm 2 lượt cho Lê Văn Cường.")).toBeInTheDocument();
    expect(sent).toEqual([
      {
        memberId: "0199ffff-0000-7000-8000-000000000004",
        body: { lessonId: "0199ffff-0000-7000-8000-000000000061", extraAttempts: 2, reason: "Mất kết nối khi đang làm bài" },
      },
    ]);
    await waitFor(() => {
      expect(screen.queryByRole("dialog")).toBeNull();
    });
    await waitFor(() => {
      expect(loads).toBe(2);
    });
  });

  it("keeps the grant dialog open with the API's refusal", async () => {
    const { user } = renderTab(fixtures.classQuizzes, [
      http.post(grantsUrl, () => apiError(409, "INVALID_TRANSITION", "Lớp không ở trạng thái đang chạy")),
    ]);

    await screen.findByRole("heading", { level: 2, name: ASSESSMENT });
    await user.click(grantButton(ASSESSMENT, "Vũ Đức Khang"));
    const dialog = await screen.findByRole("dialog", { name: "Cấp thêm lượt cho Vũ Đức Khang" });
    expect(within(dialog).getByText("Kiểm tra cuối chặng Database · đã dùng 1/2 lượt.")).toBeInTheDocument();
    await user.type(within(dialog).getByRole("textbox", { name: /^Lý do/ }), "Lỗi đề câu 4");
    await user.click(within(dialog).getByRole("button", { name: "Cấp thêm lượt" }));

    expect(await within(dialog).findByRole("alert")).toHaveTextContent("Lớp không ở trạng thái đang chạy");
  });

  it("does not grant attempts outside a running class and says why", async () => {
    const { user } = renderTab({ ...fixtures.classQuizzes, class: { ...fixtures.classQuizzes.class, status: "ended" } });

    await screen.findByRole("heading", { level: 2, name: GO });
    const button = grantButton(GO, "Lê Văn Cường");
    expect(button).toHaveAttribute("aria-disabled", "true");
    expect(button).toHaveAttribute("title", "Lớp không ở trạng thái đang chạy");
    await user.click(button);

    expect(await screen.findByText("Lớp không ở trạng thái đang chạy, không cấp thêm lượt được.")).toBeInTheDocument();
    expect(screen.queryByRole("dialog")).toBeNull();
  });

  it("lists the grants made on a quiz, newest first", async () => {
    const older: AttemptGrant = {
      ...grant,
      id: "0199ffff-0000-7000-8000-0000000000f4",
      memberId: "0199ffff-0000-7000-8000-000000000006",
      studentName: "Nguyễn Hoàng An",
      extraAttempts: 1,
      reason: "Lỗi đề câu 4",
      grantedAt: "2026-10-01T08:00:00Z",
    };
    renderTab({
      ...fixtures.classQuizzes,
      items: fixtures.classQuizzes.items.map((item) =>
        item.lesson.title === GO ? { ...item, grants: [grant, older] } : item,
      ),
    });

    await screen.findByRole("heading", { level: 2, name: GO });
    const heading = within(card(GO)).getByRole("heading", { level: 3, name: "Lịch sử cấp thêm lượt" });
    const entries = within(heading.parentElement ?? card(GO)).getAllByRole("listitem");
    expect(entries.map((li) => li.firstElementChild?.firstElementChild?.textContent)).toEqual([
      "+2 lượt cho Lê Văn Cường",
      "+1 lượt cho Nguyễn Hoàng An",
    ]);
    expect(entries[0]).toHaveTextContent("Mất kết nối khi đang làm bài");
    expect(entries[0]).toHaveTextContent("Lê Thu Hương");
    expect(entries[1]).toHaveTextContent("Lỗi đề câu 4");
    expect(within(card(ASSESSMENT)).queryByText("Lịch sử cấp thêm lượt")).toBeNull();
  });
});

const grant: AttemptGrant = {
  id: "0199ffff-0000-7000-8000-0000000000f3",
  memberId: "0199ffff-0000-7000-8000-000000000004",
  lessonId: "0199ffff-0000-7000-8000-000000000061",
  studentName: "Lê Văn Cường",
  extraAttempts: 2,
  reason: "Mất kết nối khi đang làm bài",
  grantedByName: "Lê Thu Hương",
  grantedAt: "2026-10-05T08:00:00Z",
};
