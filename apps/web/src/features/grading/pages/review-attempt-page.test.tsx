import { screen, waitFor, within } from "@testing-library/react";
import { http, HttpResponse } from "msw";
import { describe, expect, it } from "vitest";
import { summaryKeys } from "@/features/summary";
import { api, apiError, server } from "@/shared/test/msw";
import { reviewOutcome } from "../api/msw-handlers";
import type { AttemptDetail } from "../model/quiz-schemas";
import type { ReviewBody } from "../model/review-form";
import { fixtures, ids } from "../test/golden";
import { renderGrading, type RenderGradingOptions } from "../test/render-grading";

const reviewPath = (aid: string, base = "/teach") => `${base}/grading/quiz/${aid}`;
const detailUrl = api("/quiz-attempts/:aid");
const reviewsUrl = api("/quiz-attempts/:aid/reviews");
const classId = fixtures.reviewPending.class.id;
const q732 = fixtures.reviewPending.questions[1].id;

const serve = (detail: AttemptDetail) => http.get(detailUrl, () => HttpResponse.json(detail));

/** Records the posted reviews and answers with the API's outcome, `next` after a first assessment confirmation. */
function recordReviews(detail: AttemptDetail, next = fixtures.queueTeacher.items.at(0) ?? null) {
  const bodies: ReviewBody[] = [];
  const handler = http.post(reviewsUrl, async ({ request }) => {
    const body = (await request.json()) as ReviewBody;
    bodies.push(body);
    return HttpResponse.json(reviewOutcome(detail, body, next), { status: 201 });
  });
  return { handler, bodies };
}

interface PageOptions {
  role?: "teacher" | "admin";
  search?: string;
  overrides?: RenderGradingOptions["overrides"];
}

async function renderPage(detail: AttemptDetail = fixtures.reviewPending, { role, search = "", overrides = [] }: PageOptions = {}) {
  const view = renderGrading(`${reviewPath(detail.id, role === "admin" ? "/admin" : "/teach")}${search}`, {
    role,
    // Earlier handlers win: the test's own come first.
    overrides: [...overrides, serve(detail)],
  });
  await screen.findByRole("heading", { level: 1, name: /^Nguyễn Hoàng An · / });
  return view;
}

const form = () => screen.getByRole("form", { name: /Xác nhận kết quả|Nhận xét cho học viên|Sửa nhận xét/ });
const comment = () => screen.getByRole("textbox", { name: /Nhận xét chung/ });
const note = (position: number) => screen.getByRole("textbox", { name: `Ghi chú cho câu ${String(position)} (không bắt buộc)` });
const card = (title: string) => {
  const found = screen.getByRole("heading", { name: title }).closest<HTMLElement>("[data-slot=card]");
  if (!found) throw new Error(`${title} card`);
  return found;
};

describe("Quiz attempt review page", () => {
  it("shows the attempt, every answer against the key and the other attempts", async () => {
    await renderPage();

    expect(screen.getByRole("heading", { level: 1, name: "Nguyễn Hoàng An · chỉ mục" })).toBeInTheDocument();
    expect(screen.getByText("basic01 · Database · Quiz bài tập · 3 câu, đạt từ 70%")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Chấm bài" })).toHaveAttribute("href", "/teach/grading");
    expect(document.title).toContain("Quiz · Nguyễn Hoàng An");

    const facts = card("Lượt #2");
    expect(within(facts).getByText("Chờ xác nhận")).toBeInTheDocument();
    expect(within(facts).getByText("an.nguyen@gmail.com")).toBeInTheDocument();
    expect(within(facts).getByText("basic01 · Lập trình cơ bản – khóa 1")).toBeInTheDocument();
    expect(within(facts).getByText("1 phút / 10 phút")).toBeInTheDocument();
    expect(within(facts).getByText(/câu đúng/).closest("dd")).toHaveTextContent("7/10 (70%) · 2/3 câu đúng Đạt");
    expect(within(facts).getByText("#2 / 2 lượt")).toBeInTheDocument();

    const answers = card("Bài làm từng câu");
    expect(within(answers).getByText("Theo thứ tự học viên thấy")).toBeInTheDocument();
    expect(within(answers).getByText("Đúng · 4/4 điểm")).toBeInTheDocument();
    expect(within(answers).getByText("Bỏ trống · 0/3 điểm")).toBeInTheDocument();
    // The key and the picks are told to screen readers, not by colour alone.
    expect(within(answers).getByText("INSERT").closest("li")).toHaveTextContent("INSERT (đáp án đúng)");
    expect(within(answers).getByText("Truy vấn khoảng").closest("li")).toHaveTextContent(
      "Truy vấn khoảng (đáp án đúng) (học viên chọn)Học viên chọn",
    );
    expect(note(1)).toHaveAttribute("placeholder", "vd: Giải thích ngắn vì sao đúng");
    expect(note(2)).toHaveAttribute("placeholder", "vd: Xem lại phần … trong bài đọc");

    const history = card("Các lượt đã nộp");
    expect(within(history).getByText("Lượt #2 (đang xem)")).toBeInTheDocument();
    expect(within(history).getByRole("link", { name: "Lượt #1" })).toHaveAttribute("href", reviewPath(ids.attemptFirst));
    expect(within(history).getByText("3/10 · 30%")).toBeInTheDocument();
    expect(within(history).getByText("Đã có lượt mới hơn")).toBeInTheDocument();

    expect(within(form()).getByRole("heading", { name: "Xác nhận kết quả" })).toBeInTheDocument();
    expect(within(form()).getByText(/^Đã chờ /)).toBeInTheDocument();
    expect(within(form()).getByText((_, el) => el?.textContent === "Điểm tự động 7/10 (70%)")).toBeInTheDocument();
    expect(within(form()).getByText(/giảng viên không sửa điểm/)).toHaveTextContent(
      "Điểm tự động đã tính vào tiến độ; giảng viên không sửa điểm. Xác nhận để ghi nhận đã xem bài và gửi nhận xét cho học viên.",
    );
    expect(within(form()).getByRole("link", { name: "Về hàng chờ" })).toHaveAttribute("href", "/teach/grading");
  });

  it("moves on to the next queue item after a first confirmation", async () => {
    const recorded = recordReviews(fixtures.reviewPending);
    const next = fixtures.queueTeacher.items.at(0);
    const { user, router, queryClient } = await renderPage(fixtures.reviewPending, { overrides: [recorded.handler] });
    // A class summary seen before the save shows the old cell until it reloads.
    const matrixKey = summaryKeys.matrix(ids.classId);
    queryClient.setQueryData(matrixKey, {});

    await user.type(note(2), "  Xem lại chi phí ghi. ");
    await user.type(comment(), "Đạt ngưỡng.");
    // The next item answers like any submission: the page stays on it.
    server.use(http.get(api("/submissions/:sid"), () => HttpResponse.json(fixtures.submissionPending)));
    await user.click(screen.getByRole("button", { name: "Xác nhận" }));

    expect(
      await screen.findByText("Đã xác nhận kết quả. Email nhận xét đã gửi cho học viên. Chuyển sang mục tiếp theo."),
    ).toBeInTheDocument();
    expect(recorded.bodies).toEqual([{ comment: "Đạt ngưỡng.", notes: [{ questionId: q732, note: "Xem lại chi phí ghi." }] }]);
    expect(queryClient.getQueryState(matrixKey)?.isInvalidated).toBe(true);
    await waitFor(() => {
      expect(router.state.location.pathname).toBe(`/teach/grading/${next?.itemId ?? ""}`);
    });
  });

  it("goes back to the queue when nothing is left to confirm", async () => {
    const recorded = recordReviews(fixtures.reviewPending, null);
    const { user, router } = await renderPage(fixtures.reviewPending, { overrides: [recorded.handler] });

    await user.type(comment(), "Đạt ngưỡng.");
    await user.click(screen.getByRole("button", { name: "Xác nhận" }));

    expect(
      await screen.findByText("Đã xác nhận kết quả. Email nhận xét đã gửi cho học viên. Đã xử lý hết hàng chờ."),
    ).toBeInTheDocument();
    await waitFor(() => {
      expect(router.state.location.pathname).toBe("/teach/grading");
    });
    expect(await screen.findByRole("heading", { level: 1, name: "Chấm bài" })).toBeInTheDocument();
  });

  it("edits the current review in place", async () => {
    const recorded = recordReviews(fixtures.reviewConfirmed);
    const { user, router } = await renderPage(fixtures.reviewConfirmed, { overrides: [recorded.handler] });

    expect(within(form()).getByRole("heading", { name: "Sửa nhận xét" })).toBeInTheDocument();
    expect(within(form()).queryByText(/^Đã chờ /)).toBeNull();
    expect(within(form()).getByText(/^Đã xác nhận bởi Lê Thu Hương · /)).toHaveTextContent(/ · đã sửa 1 lần$/);
    expect(comment()).toHaveValue("Đạt ngưỡng. Lượt sau chú ý câu về chỉ mục kết hợp.");
    expect(note(1)).toHaveValue("Tiền tố trái quyết định chỉ mục có dùng được không.");
    expect(within(card("Các lượt đã nộp")).getByText(/^Lê Thu Hương: "Đạt ngưỡng\. Lượt sau/)).toBeInTheDocument();
    await user.clear(note(1));
    await user.click(screen.getByRole("button", { name: "Lưu nhận xét mới" }));

    expect(await screen.findByText("Đã lưu nhận xét mới. Học viên nhận email cập nhật.")).toBeInTheDocument();
    expect(recorded.bodies).toEqual([{ comment: "Đạt ngưỡng. Lượt sau chú ý câu về chỉ mục kết hợp.", notes: [] }]);
    expect(router.state.location.pathname).toBe(reviewPath(ids.attemptLatest));
  });

  it("comments a practice attempt and stays on it, with the way back to the class's quiz stats", async () => {
    const recorded = recordReviews(fixtures.practiceUnreviewed);
    const { user, router } = await renderPage(fixtures.practiceUnreviewed, { overrides: [recorded.handler] });

    expect(screen.getByText("basic01 · Database · Quiz học liệu · 3 câu")).toBeInTheDocument();
    const crumbs = screen.getByRole("navigation", { name: "Đường dẫn" });
    expect(within(crumbs).getByRole("link", { name: "Lớp của tôi" })).toHaveAttribute("href", "/teach");
    expect(within(crumbs).getByRole("link", { name: "basic01" })).toHaveAttribute(
      "href",
      `/teach/classes/${classId}?tab=quiz`,
    );
    expect(within(crumbs).getByText("Nguyễn Hoàng An lượt #1")).toHaveAttribute("aria-current", "page");
    const facts = card("Lượt #1");
    expect(within(facts).getByText("Chưa nhận xét")).toBeInTheDocument();
    expect(within(facts).getByText("#1 · không giới hạn lượt")).toBeInTheDocument();
    expect(within(facts).getByText("1 phút")).toBeInTheDocument();
    expect(within(facts).queryByText(/Đạt|Chưa đạt/)).toBeNull();
    expect(within(card("Bài làm từng câu")).getByText("Sai · 0/3 điểm")).toBeInTheDocument();
    expect(within(form()).getByRole("heading", { name: "Nhận xét cho học viên" })).toBeInTheDocument();
    expect(within(form()).getByText("Quiz học liệu")).toBeInTheDocument();
    expect(within(form()).getByText(/Nhận xét là tùy chọn với quiz học liệu\.$/)).toBeInTheDocument();
    expect(within(form()).getByRole("link", { name: "Về thống kê quiz" })).toHaveAttribute(
      "href",
      `/teach/classes/${classId}?tab=quiz`,
    );

    await user.type(note(2), "UNIQUE cũng chặn trùng.");
    await user.type(comment(), "Câu ràng buộc còn thiếu một đáp án.");
    await user.click(screen.getByRole("button", { name: "Gửi nhận xét" }));

    expect(await screen.findByText("Đã gửi nhận xét cho học viên.")).toBeInTheDocument();
    expect(recorded.bodies).toEqual([
      {
        comment: "Câu ràng buộc còn thiếu một đáp án.",
        notes: [{ questionId: fixtures.practiceUnreviewed.questions[1].id, note: "UNIQUE cũng chặn trùng." }],
      },
    ]);
    expect(router.state.location.pathname).toBe(reviewPath(fixtures.practiceUnreviewed.id));
  });

  it("asks for the general comment before sending", async () => {
    const recorded = recordReviews(fixtures.reviewPending);
    const { user } = await renderPage(fixtures.reviewPending, { overrides: [recorded.handler] });

    await user.type(note(1), "Đúng.");
    await user.click(screen.getByRole("button", { name: "Xác nhận" }));

    expect(await screen.findByText("Nhập nhận xét chung cho học viên.")).toBeInTheDocument();
    expect(comment()).toHaveAttribute("aria-invalid", "true");
    expect(recorded.bodies).toHaveLength(0);
  });

  it("puts the API's comment check on the comment and its other refusals on the form", async () => {
    let message = "Nhận xét tối đa 5000 ký tự.";
    const { user } = await renderPage(fixtures.reviewPending, {
      overrides: [http.post(reviewsUrl, () => apiError(400, "VALIDATION_FAILED", message))],
    });

    await user.type(comment(), "Tốt.");
    await user.click(screen.getByRole("button", { name: "Xác nhận" }));
    expect(await screen.findByText("Nhận xét tối đa 5000 ký tự.")).toBeInTheDocument();
    expect(comment()).toHaveAttribute("aria-invalid", "true");
    expect(within(form()).queryByRole("alert")).toBeNull();

    message = "Ghi chú cho câu không thuộc lượt làm bài này.";
    await user.click(screen.getByRole("button", { name: "Xác nhận" }));
    expect(await within(form()).findByRole("alert")).toHaveTextContent("Ghi chú cho câu không thuộc lượt làm bài này.");
  });

  it("shows the API's refusal outside the grader's classes", async () => {
    const { user } = await renderPage(fixtures.reviewPending, {
      overrides: [http.post(reviewsUrl, () => apiError(403, "FORBIDDEN", "Bạn chỉ nhận xét bài của lớp mình phụ trách."))],
    });

    await user.type(comment(), "Tốt.");
    await user.click(screen.getByRole("button", { name: "Xác nhận" }));

    expect(await within(form()).findByRole("alert")).toHaveTextContent("Bạn chỉ nhận xét bài của lớp mình phụ trách.");
  });

  it("locks the form when a newer attempt came in meanwhile", async () => {
    const [latest, first] = fixtures.reviewPending.attempts;
    const newer = { ...latest, id: "0199ffff-0000-7000-8000-0000000000aa", no: 3 };
    const overtaken: AttemptDetail = {
      ...fixtures.reviewPending,
      reviewStatus: "superseded",
      isLatest: false,
      latestAttemptId: newer.id,
      attempts: [newer, { ...latest, reviewStatus: "superseded" }, first],
    };
    let loads = 0;
    const { user } = await renderPage(fixtures.reviewPending, {
      overrides: [
        http.get(detailUrl, () => {
          loads += 1;
          return HttpResponse.json(loads === 1 ? fixtures.reviewPending : overtaken);
        }),
        http.post(reviewsUrl, () =>
          apiError(409, "CONFLICT", "Học viên đã nộp lượt mới hơn. Hãy xác nhận lượt mới nhất."),
        ),
      ],
    });

    await user.type(comment(), "Tốt.");
    await user.click(screen.getByRole("button", { name: "Xác nhận" }));

    expect(await within(form()).findByRole("alert")).toHaveTextContent(
      "Học viên đã nộp lượt mới hơn. Hãy xác nhận lượt mới nhất.",
    );
    expect(await screen.findByRole("link", { name: "Mở lượt #3" })).toHaveAttribute("href", reviewPath(newer.id));
    expect(screen.getByRole("button", { name: "Xác nhận" })).toBeDisabled();
  });

  it("locks a superseded attempt and links to the latest", async () => {
    await renderPage(fixtures.reviewSuperseded);

    expect(
      screen.getByText(/^Học viên đã nộp lượt #2; lượt này không cần xác nhận nữa\./),
    ).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Mở lượt #2" })).toHaveAttribute("href", reviewPath(ids.attemptLatest));
    expect(within(card("Lượt #1")).getByText("Chưa đạt · cần 70%")).toBeInTheDocument();
    expect(within(card("Bài làm từng câu")).getByText("Sai · 0/4 điểm")).toBeInTheDocument();
    expect(screen.getByRole("group", { name: "Nhận xét lượt #1" })).toBeDisabled();
    expect(screen.getByRole("group", { name: "Ghi chú từng câu" })).toBeDisabled();
    expect(note(1)).toBeDisabled();
    expect(screen.getByRole("button", { name: "Xác nhận" })).toBeDisabled();
    expect(within(form()).queryByText(/^Đã chờ /)).toBeNull();
  });

  it("leaves for the queue when the attempt is not in the grader's classes", async () => {
    const { router } = renderGrading(reviewPath(ids.attemptLatest), {
      overrides: [http.get(detailUrl, () => apiError(403, "FORBIDDEN", "Bạn chỉ xem được lớp mình phụ trách."))],
    });

    expect(await screen.findByText("Bạn chỉ xem được lớp mình phụ trách.")).toBeInTheDocument();
    await waitFor(() => {
      expect(router.state.location.pathname).toBe("/teach/grading");
    });
  });

  it("leaves for the queue when the attempt does not exist", async () => {
    const { router } = renderGrading(reviewPath("0199ffff-0000-7000-8000-0000000000ff"));

    expect(await screen.findByText("Không tìm thấy lượt làm bài đã nộp.")).toBeInTheDocument();
    await waitFor(() => {
      expect(router.state.location.pathname).toBe("/teach/grading");
    });
  });

  it("keeps an admin in the admin area", async () => {
    await renderPage(fixtures.reviewPending, { role: "admin" });

    expect(screen.getByRole("link", { name: "Chấm bài" })).toHaveAttribute("href", "/admin/grading");
    expect(screen.getByRole("link", { name: "Về hàng chờ" })).toHaveAttribute("href", "/admin/grading");
    expect(within(card("Các lượt đã nộp")).getByRole("link", { name: "Lượt #1" })).toHaveAttribute(
      "href",
      reviewPath(ids.attemptFirst, "/admin"),
    );
  });

  it("leads back to the class's quiz stats when opened from there", async () => {
    await renderPage(fixtures.reviewPending, { role: "admin", search: "?from=quiz" });

    const crumbs = screen.getByRole("navigation", { name: "Đường dẫn" });
    expect(within(crumbs).getByRole("link", { name: "Lớp học" })).toHaveAttribute("href", "/admin/classes");
    expect(within(crumbs).getByRole("link", { name: "basic01" })).toHaveAttribute(
      "href",
      `/admin/classes/${classId}?tab=quiz`,
    );
    expect(within(form()).getByRole("link", { name: "Về thống kê quiz" })).toHaveAttribute(
      "href",
      `/admin/classes/${classId}?tab=quiz`,
    );
    expect(within(card("Các lượt đã nộp")).getByRole("link", { name: "Lượt #1" })).toHaveAttribute(
      "href",
      `${reviewPath(ids.attemptFirst, "/admin")}?from=quiz`,
    );
  });
});
