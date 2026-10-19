import { screen, waitFor, within } from "@testing-library/react";
import { http, HttpResponse } from "msw";
import { describe, expect, it } from "vitest";
import { summaryKeys } from "@/features/summary";
import { api, apiError, server } from "@/shared/test/msw";
import { gradeOutcome } from "../api/msw-handlers";
import type { GradeBody } from "../model/grade-form";
import type { SubmissionDetail } from "../model/schemas";
import { fixtures, ids } from "../test/golden";
import { renderGrading } from "../test/render-grading";

const gradePath = (sid: string, base = "/teach") => `${base}/grading/${sid}`;
const detailUrl = api("/submissions/:sid");
const gradesUrl = api("/submissions/:sid/grades");

const serve = (detail: SubmissionDetail) => http.get(detailUrl, () => HttpResponse.json(detail));

/** Records the graded bodies and answers with the API's outcome, `next` after a first grading. */
function recordGrades(detail: SubmissionDetail, next = fixtures.queueTeacher.items.at(0) ?? null) {
  const bodies: GradeBody[] = [];
  const handler = http.post(gradesUrl, async ({ request }) => {
    const body = (await request.json()) as GradeBody;
    bodies.push(body);
    return HttpResponse.json(gradeOutcome(detail, body, next), { status: 201 });
  });
  return { handler, bodies };
}

async function renderPage(detail: SubmissionDetail = fixtures.submissionPending, overrides: Parameters<typeof renderGrading>[1] = {}) {
  const view = renderGrading(gradePath(detail.submission.id, overrides.role === "admin" ? "/admin" : "/teach"), {
    ...overrides,
    overrides: [serve(detail), ...(overrides.overrides ?? [])],
  });
  await screen.findByRole("heading", { level: 1, name: "Nguyễn Hoàng An · truy vấn SELECT" });
  return view;
}

const form = () => screen.getByRole("form", { name: /Chấm theo tiêu chí|Sửa kết quả chấm/ });
const pointsInput = (name: string) => within(form()).getByRole("spinbutton", { name });

describe("Grade submission page", () => {
  it("shows the submission, the brief and the history next to the rubric", async () => {
    await renderPage();

    expect(screen.getByText("basic01 · Database · 2 tiêu chí, tổng 100 điểm")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Chấm bài" })).toHaveAttribute("href", "/teach/grading");
    expect(screen.getByRole("heading", { name: "Bài nộp #2" })).toBeInTheDocument();
    expect(screen.getByText(/^Chờ chấm · /)).toBeInTheDocument();
    expect(screen.getByRole("link", { name: /github\.com\/an\/sql-select/ })).toHaveAttribute("target", "_blank");
    expect(screen.getByText("d4e5f6a")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: /giai-thich\.md/ })).toBeInTheDocument();
    expect(screen.getByText("Đề bài: truy vấn SELECT")).toBeInTheDocument();
    expect(document.title).toContain("Chấm bài · Nguyễn Hoàng An");

    const history = screen.getByRole("heading", { name: "Lịch sử nộp và chấm" }).closest<HTMLElement>("[data-slot=card]");
    if (!history) throw new Error("history card");
    expect(within(history).getByText("Lần nộp #2 (đang xem)")).toBeInTheDocument();
    expect(within(history).getByRole("link", { name: "Lần nộp #1" })).toHaveAttribute("href", gradePath(ids.first));
    expect(within(history).getByText(/Lê Thu Hương: 50\/100 · Cần làm lại/)).toBeInTheDocument();
    const scores = within(history).getByRole("list", { name: "Điểm từng tiêu chí" });
    expect(within(scores).getByText(/Truy vấn đúng kết quả:/)).toHaveTextContent("Truy vấn đúng kết quả: 30/40");
    expect(within(scores).getByText(/Giải thích cách làm:/)).toHaveTextContent(
      "Giải thích cách làm: 20/60 · Nên nêu lý do chọn JOIN.",
    );
  });

  it("totals the points as they are typed and gives the verdict at the threshold", async () => {
    const { user } = await renderPage();

    expect(screen.getByText("Đạt từ 70% (70/100 điểm)")).toBeInTheDocument();
    expect(screen.getByText("Còn 2 tiêu chí chưa nhập")).toBeInTheDocument();
    await user.type(pointsInput("Truy vấn đúng kết quả"), "40");
    expect(screen.getByText("Còn 1 tiêu chí chưa nhập")).toBeInTheDocument();
    await user.type(pointsInput("Giải thích cách làm"), "29");

    const total = screen.getByText(/^Tổng/).closest<HTMLElement>("[aria-live=polite]");
    if (!total) throw new Error("live total");
    expect(total).toHaveTextContent("Tổng 69/100 (69%)");
    expect(within(total).getByText("Cần làm lại")).toBeInTheDocument();
    await user.clear(pointsInput("Giải thích cách làm"));
    await user.type(pointsInput("Giải thích cách làm"), "30");
    expect(total).toHaveTextContent("Tổng 70/100 (70%)");
    expect(within(total).getByText("Đạt")).toBeInTheDocument();
  });

  it("asks for every criterion and the comment before sending", async () => {
    const recorded = recordGrades(fixtures.submissionPending);
    const { user } = await renderPage(fixtures.submissionPending, { overrides: [recorded.handler] });

    await user.type(pointsInput("Giải thích cách làm"), "61");
    await user.click(screen.getByRole("button", { name: "Lưu kết quả" }));

    expect(await screen.findByText('Nhập điểm cho tiêu chí "Truy vấn đúng kết quả".')).toBeInTheDocument();
    expect(screen.getByText('Điểm tiêu chí "Giải thích cách làm" phải từ 0 đến 60.')).toBeInTheDocument();
    expect(screen.getByText("Nhập nhận xét chung cho học viên.")).toBeInTheDocument();
    expect(pointsInput("Truy vấn đúng kết quả")).toHaveAttribute("aria-invalid", "true");
    expect(screen.getByRole("textbox", { name: /Nhận xét chung/ })).toHaveAttribute("aria-invalid", "true");
    expect(recorded.bodies).toHaveLength(0);
  });

  it("moves on to the next queue item after a first grading", async () => {
    const recorded = recordGrades(fixtures.submissionPending);
    const next = fixtures.queueTeacher.items.at(0);
    const { user, router, queryClient } = await renderPage(fixtures.submissionPending, {
      overrides: [recorded.handler],
    });
    // A class summary seen before the save shows the old cell until it reloads.
    const matrixKey = summaryKeys.matrix(ids.classId);
    queryClient.setQueryData(matrixKey, {});

    await user.type(pointsInput("Truy vấn đúng kết quả"), "40");
    await user.type(pointsInput("Giải thích cách làm"), "30");
    await user.type(within(form()).getByRole("textbox", { name: "Ghi chú: Giải thích cách làm" }), "Thiếu ví dụ.");
    await user.type(screen.getByRole("textbox", { name: /Nhận xét chung/ }), "Đạt yêu cầu.");
    // The next item answers like any submission: the page stays on the grading page of it.
    server.use(serve(fixtures.submissionPending));
    await user.click(screen.getByRole("button", { name: "Lưu kết quả" }));

    expect(
      await screen.findByText(
        "Đã lưu 70/100 · Đạt. Email kết quả đã gửi cho học viên. Chuyển sang bài tiếp theo.",
      ),
    ).toBeInTheDocument();
    expect(recorded.bodies).toEqual([
      {
        scores: [
          { criterionId: "01990000-0000-7000-8000-000000000932", points: 40, comment: "" },
          { criterionId: "01990000-0000-7000-8000-000000000934", points: 30, comment: "Thiếu ví dụ." },
        ],
        comment: "Đạt yêu cầu.",
      },
    ]);
    expect(queryClient.getQueryState(matrixKey)?.isInvalidated).toBe(true);
    await waitFor(() => {
      expect(router.state.location.pathname).toBe(gradePath(next?.itemId ?? ""));
    });
  });

  it("goes back to the queue when nothing is left to grade", async () => {
    const recorded = recordGrades(fixtures.submissionPending, null);
    const { user, router } = await renderPage(fixtures.submissionPending, { overrides: [recorded.handler] });

    await user.type(pointsInput("Truy vấn đúng kết quả"), "20");
    await user.type(pointsInput("Giải thích cách làm"), "20");
    await user.type(screen.getByRole("textbox", { name: /Nhận xét chung/ }), "Cần làm lại phần giải thích.");
    await user.click(screen.getByRole("button", { name: "Lưu kết quả" }));

    expect(
      await screen.findByText("Đã lưu 40/100 · Cần làm lại. Email kết quả đã gửi cho học viên. Đã chấm hết hàng chờ."),
    ).toBeInTheDocument();
    await waitFor(() => {
      expect(router.state.location.pathname).toBe("/teach/grading");
    });
    expect(await screen.findByRole("heading", { level: 1, name: "Chấm bài" })).toBeInTheDocument();
  });

  it("edits the current grading in place", async () => {
    const recorded = recordGrades(fixtures.submissionPassed);
    const { user, router } = await renderPage(fixtures.submissionPassed, { overrides: [recorded.handler] });

    expect(screen.getByRole("heading", { name: "Sửa kết quả chấm" })).toBeInTheDocument();
    expect(pointsInput("Giải thích cách làm")).toHaveValue(45);
    expect(screen.getByRole("textbox", { name: /Nhận xét chung/ })).toHaveValue("Đã bổ sung giải thích, đạt yêu cầu.");
    await user.clear(pointsInput("Giải thích cách làm"));
    await user.type(pointsInput("Giải thích cách làm"), "55");
    await user.click(screen.getByRole("button", { name: "Lưu kết quả mới" }));

    expect(await screen.findByText("Đã lưu 95/100 · Đạt. Email kết quả đã gửi cho học viên.")).toBeInTheDocument();
    expect(recorded.bodies.at(0)?.scores.at(1)).toMatchObject({ points: 55, comment: "Nên nêu lý do chọn JOIN." });
    expect(router.state.location.pathname).toBe(gradePath(ids.latest));
  });

  it("shows the API's refusal when a newer submission came in meanwhile", async () => {
    const { user } = await renderPage(fixtures.submissionPending, {
      overrides: [
        http.post(gradesUrl, () => apiError(409, "SUPERSEDED", "Học viên đã nộp lần mới hơn. Chỉ chấm lần nộp mới nhất.")),
      ],
    });

    await user.type(pointsInput("Truy vấn đúng kết quả"), "40");
    await user.type(pointsInput("Giải thích cách làm"), "40");
    await user.type(screen.getByRole("textbox", { name: /Nhận xét chung/ }), "Tốt.");
    await user.click(screen.getByRole("button", { name: "Lưu kết quả" }));

    expect(await within(form()).findByRole("alert")).toHaveTextContent(
      "Học viên đã nộp lần mới hơn. Chỉ chấm lần nộp mới nhất.",
    );
  });

  it("puts the API's field errors next to the criterion", async () => {
    const { user } = await renderPage(fixtures.submissionPending, {
      overrides: [
        http.post(gradesUrl, () =>
          HttpResponse.json(
            {
              error: {
                code: "VALIDATION",
                message: "Dữ liệu không hợp lệ.",
                details: { "scores.01990000-0000-7000-8000-000000000934": "Điểm vượt mức tối đa." },
              },
            },
            { status: 422 },
          ),
        ),
      ],
    });

    await user.type(pointsInput("Truy vấn đúng kết quả"), "40");
    await user.type(pointsInput("Giải thích cách làm"), "40");
    await user.type(screen.getByRole("textbox", { name: /Nhận xét chung/ }), "Tốt.");
    await user.click(screen.getByRole("button", { name: "Lưu kết quả" }));

    expect(await screen.findByText("Điểm vượt mức tối đa.")).toBeInTheDocument();
    expect(pointsInput("Giải thích cách làm")).toHaveAttribute("aria-invalid", "true");
    expect(within(form()).queryByRole("alert")).toBeNull();
  });

  it("locks an older submission and links to the latest", async () => {
    await renderPage(fixtures.submissionOlder);

    expect(screen.getByText(/Đây là lần nộp #1; học viên đã nộp lần #2\. Chỉ chấm lần nộp mới nhất\./)).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Mở lần nộp #2" })).toHaveAttribute("href", gradePath(ids.latest));
    expect(within(form()).getByRole("group", { name: "Điểm từng tiêu chí" })).toBeDisabled();
    expect(screen.getByRole("button", { name: "Lưu kết quả mới" })).toBeDisabled();
  });

  it("leaves for the queue when the submission is not the grader's", async () => {
    const { router } = renderGrading(gradePath(ids.latest), {
      overrides: [http.get(detailUrl, () => apiError(403, "FORBIDDEN", "Bạn chỉ chấm bài của lớp mình phụ trách."))],
    });

    expect(await screen.findByText("Bạn chỉ chấm bài của lớp mình phụ trách.")).toBeInTheDocument();
    await waitFor(() => {
      expect(router.state.location.pathname).toBe("/teach/grading");
    });
  });

  it("leaves for the queue when the submission does not exist", async () => {
    const { router } = renderGrading(gradePath("0199ffff-0000-7000-8000-0000000000ff"));

    expect(await screen.findByText("Không tìm thấy bài nộp.")).toBeInTheDocument();
    await waitFor(() => {
      expect(router.state.location.pathname).toBe("/teach/grading");
    });
  });

  it("keeps an admin in the admin area", async () => {
    await renderPage(fixtures.submissionPending, { role: "admin" });

    expect(screen.getByRole("link", { name: "Chấm bài" })).toHaveAttribute("href", "/admin/grading");
    expect(screen.getByRole("link", { name: "Về hàng chờ" })).toHaveAttribute("href", "/admin/grading");
  });
});
