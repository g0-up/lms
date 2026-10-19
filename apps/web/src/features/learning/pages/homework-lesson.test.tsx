import { screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { http, HttpResponse, type RequestHandler } from "msw";
import { beforeAll, describe, expect, it } from "vitest";
import { api, apiError } from "@/shared/test/msw";
import { submitted } from "../api/msw-handlers";
import type { MySubmissions } from "../model/schemas";
import type { SubmitBody } from "../model/submission-form";
import { fixtures, ids } from "../test/golden";
import { lessonPath, renderLearn } from "../test/render-learn";

// The route is lazy: load it once up front so the first test's timeouts do not include the import.
beforeAll(async () => {
  await import("./lesson-page");
});

const submissionsUrl = api("/me/classes/:classId/lessons/:lessonId/submissions");
const roadmap = http.get(api("/me/classes/:classId"), () => HttpResponse.json(fixtures.roadmapWithHomework));
const withSubmissions = (data: MySubmissions) => http.get(submissionsUrl, () => HttpResponse.json(data));

async function renderHomework(overrides: RequestHandler[] = []) {
  const view = renderLearn(lessonPath(ids.active, ids.homework), { overrides: [roadmap, ...overrides] });
  await screen.findByRole("heading", { level: 1, name: "Bài tập: truy vấn SELECT" });
  return view;
}

const mdFile = (name = "giai-thich.md") => new File(["# Giải thích"], name, { type: "text/markdown" });

describe("Homework lesson", () => {
  it("shows the brief, the attached file as a download and the rubric, without a tick", async () => {
    await renderHomework();

    expect(await screen.findByRole("heading", { level: 2, name: "Nộp bài" })).toBeInTheDocument();
    const download = screen.getByRole("link", { name: /du-lieu-mau\.sql/ });
    expect(download).toHaveAttribute("href", "/api/v1/media/0199ffff-0000-7000-8000-0000000000a1/content");
    expect(download).toHaveAttribute("download", "du-lieu-mau.sql");
    expect(download).toHaveTextContent("2 KB");
    expect(screen.getByRole("heading", { name: "Tiêu chí chấm · đạt từ 70% (70/100 điểm)" })).toBeInTheDocument();
    expect(screen.getByText("Truy vấn đúng kết quả")).toBeInTheDocument();
    expect(screen.getByText("60 điểm")).toBeInTheDocument();

    expect(screen.queryByRole("button", { name: "Đã học xong" })).toBeNull();
    expect(screen.getByRole("link", { name: "Học liệu trước" })).toBeInTheDocument();
    expect(screen.queryByRole("heading", { name: "Lịch sử nộp" })).toBeNull();
    expect(screen.queryByRole("heading", { name: /Kết quả lần nộp/ })).toBeNull();
  });

  it("asks for a public repo", async () => {
    await renderHomework();

    const repo = await screen.findByLabelText("Đường dẫn repo");
    expect(repo).toHaveAccessibleDescription("GitHub, GitLab hoặc Bitbucket. Repo phải để public để giảng viên mở được.");
    expect(screen.getByLabelText("File đính kèm")).toHaveAccessibleDescription(
      "Tối đa 5 file, mỗi file 20 MB: zip, pdf, md, txt, sql, png, jpg, jpeg. Cần repo hoặc ít nhất một file.",
    );
  });

  it("checks the form before uploading anything", async () => {
    let posts = 0;
    const { user } = await renderHomework([
      http.post(api("/me/uploads"), () => {
        posts += 1;
        return HttpResponse.json({}, { status: 500 });
      }),
    ]);
    const submit = await screen.findByRole("button", { name: "Nộp bài" });

    await user.click(submit);
    expect(await screen.findByText("Nộp đường dẫn repo hoặc ít nhất một file.")).toBeInTheDocument();

    await user.type(screen.getByLabelText("Đường dẫn repo"), "https://example.com/an/repo");
    await user.click(submit);
    expect(
      await screen.findByText(
        "Đường dẫn repo phải là https trên GitHub, GitLab hoặc Bitbucket, ví dụ https://github.com/ten-ban/ten-repo.",
      ),
    ).toBeInTheDocument();

    await user.clear(screen.getByLabelText("Đường dẫn repo"));
    const files = screen.getByLabelText("File đính kèm");
    // The picker's accept list filters by default; a browser lets the student pick "All files".
    await userEvent.setup({ applyAccept: false }).upload(files, new File(["x"], "setup.exe"));
    await user.click(submit);
    expect(
      await screen.findByText("File setup.exe không đúng định dạng. Chấp nhận: zip, pdf, md, txt, sql, png, jpg, jpeg."),
    ).toBeInTheDocument();

    const six = ["1", "2", "3", "4", "5", "6"].map((n) => mdFile(`${n}.md`));
    await user.upload(files, six);
    await user.click(submit);
    expect(await screen.findByText("Tối đa 5 file mỗi lần nộp.")).toBeInTheDocument();
    expect(posts).toBe(0);
  });

  it("uploads the file as the student's document, then submits it with the repo", async () => {
    const reserved: unknown[] = [];
    let body: SubmitBody | undefined;
    const { user } = await renderHomework([
      http.post(api("/me/uploads"), async ({ request }) => {
        reserved.push(await request.json());
        return HttpResponse.json(
          {
            mediaId: "0199ffff-0000-7000-8000-0000000000f1",
            uploadUrl: "https://storage.mem/put/homework?X-Amz-Signature=mem",
            expiresAt: new Date(Date.now() + 600_000).toISOString(),
            headers: { "Content-Type": "text/plain; charset=utf-8" },
          },
          { status: 201 },
        );
      }),
      http.post(submissionsUrl, async ({ request }) => {
        body = (await request.json()) as SubmitBody;
        return HttpResponse.json(submitted(fixtures.mySubmissionsNone, body), { status: 201 });
      }),
    ]);

    await user.type(await screen.findByLabelText("Đường dẫn repo"), "https://github.com/an/sql-select");
    await user.type(screen.getByLabelText("Commit hoặc tag"), "v1.0");
    await user.upload(screen.getByLabelText("File đính kèm"), mdFile());
    expect(within(screen.getByRole("list", { name: "File đã chọn" })).getByText(/giai-thich\.md/)).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "Nộp bài" }));

    expect(await screen.findByText("Đã nộp lần #1. Bạn sẽ nhận email khi có kết quả.")).toBeInTheDocument();
    expect(reserved).toEqual([
      { kind: "document", fileName: "giai-thich.md", contentType: "text/markdown", sizeBytes: 15 },
    ]);
    expect(body).toEqual({
      repoUrl: "https://github.com/an/sql-select",
      commitRef: "v1.0",
      mediaIds: ["0199ffff-0000-7000-8000-0000000000f1"],
    });

    expect(screen.getByRole("heading", { level: 2, name: "Nộp lại (lần #2)" })).toBeInTheDocument();
    expect(screen.getByText(/Lần nộp #1 đang chờ chấm\. Nộp lại sẽ thay lần nộp đó/)).toBeInTheDocument();
    expect(screen.getByLabelText("Đường dẫn repo")).toHaveValue("https://github.com/an/sql-select");
    expect(screen.queryByRole("list", { name: "File đã chọn" })).toBeNull();
    const history = within(screen.getByRole("heading", { name: "Lịch sử nộp" }).closest("[data-slot=card]") as HTMLElement);
    expect(history.getByText("Lần nộp #1")).toBeInTheDocument();
    expect(history.getByText("Chờ chấm")).toBeInTheDocument();
  });

  it("relabels the homework in the side list after a resubmission", async () => {
    let submittedOnce = false;
    const view = renderLearn(lessonPath(ids.active, ids.homework), {
      overrides: [
        http.get(api("/me/classes/:classId"), () =>
          HttpResponse.json(submittedOnce ? fixtures.roadmapHomeworkPending : fixtures.roadmapHomeworkRework),
        ),
        withSubmissions(fixtures.mySubmissions),
        http.post(submissionsUrl, async ({ request }) => {
          submittedOnce = true;
          return HttpResponse.json(submitted(fixtures.mySubmissions, (await request.json()) as SubmitBody, true), {
            status: 201,
          });
        }),
      ],
    });
    const sideList = await screen.findByRole("complementary", { name: /^Học liệu chặng / });
    const homeworkLink = await within(sideList).findByRole("link", { name: /^Bài tập: truy vấn SELECT/ });
    expect(homeworkLink).toHaveAccessibleName("Bài tập: truy vấn SELECT Cần làm lại");

    await view.user.click(await screen.findByRole("button", { name: "Nộp lại" }));

    await waitFor(() => {
      expect(homeworkLink).toHaveAccessibleName("Bài tập: truy vấn SELECT Chờ chấm");
    });
  });

  it("replaces a pending submission and labels it in the history", async () => {
    const { user } = await renderHomework([
      withSubmissions(fixtures.mySubmissions),
      http.post(submissionsUrl, async ({ request }) =>
        HttpResponse.json(submitted(fixtures.mySubmissions, (await request.json()) as SubmitBody, true), {
          status: 201,
        }),
      ),
    ]);

    // #1 was graded Cần làm lại; #2 waits, so the result says a newer submission is being graded.
    const feedback = within(
      (await screen.findByRole("heading", { name: "Kết quả lần nộp #1" })).closest("[data-slot=card]") as HTMLElement,
    );
    expect(feedback.getByText("Chờ chấm lần nộp mới")).toBeInTheDocument();
    expect(feedback.getByText("50/100")).toBeInTheDocument();
    expect(feedback.getByRole("row", { name: "Giải thích cách làm 20/60 Nên nêu lý do chọn JOIN." })).toBeInTheDocument();
    expect(feedback.getByText("Nhận xét của Lê Thu Hương")).toBeInTheDocument();
    expect(feedback.getByText("Truy vấn đúng nhưng thiếu phần giải thích.")).toBeInTheDocument();

    // The golden deadline has passed: the student may still submit, marked late.
    expect(screen.getByText("Đã quá hạn nộp. Bạn vẫn nộp được, bài sẽ được đánh dấu nộp muộn.")).toBeInTheDocument();
    expect(screen.getByText(/^Hạn 05\/10\/2026 15:00 · quá hạn/)).toBeInTheDocument();
    expect(screen.getByLabelText("Đường dẫn repo")).toHaveValue("https://github.com/an/sql-select");
    expect(screen.getByLabelText("Commit hoặc tag")).toHaveValue("d4e5f6a");

    await user.click(screen.getByRole("button", { name: "Nộp lại" }));

    expect(
      await screen.findByText(
        "Đã nộp lần #3 (nộp muộn), thay lần #2 đang chờ chấm. Bạn sẽ nhận email khi có kết quả.",
      ),
    ).toBeInTheDocument();
    const history = within(screen.getByRole("heading", { name: "Lịch sử nộp" }).closest("[data-slot=card]") as HTMLElement);
    const rows = history.getAllByRole("listitem");
    expect(rows.map((row) => row.textContent)).toEqual([
      expect.stringMatching(/^Lần nộp #3.*Chờ chấm$/),
      expect.stringMatching(/^Lần nộp #2.*nộp muộn.*github\.com\/an\/sql-select.*giai-thich\.md.*Đã thay bằng lần nộp sau$/),
      expect.stringMatching(/^Lần nộp #1.*50\/100 điểm.*Cần làm lại$/),
    ]);
  });

  it("shows the passing result and no form once the homework passed", async () => {
    await renderHomework([withSubmissions(fixtures.mySubmissionsPassed)]);

    const feedback = within(
      (await screen.findByRole("heading", { name: "Kết quả lần nộp #2" })).closest("[data-slot=card]") as HTMLElement,
    );
    expect(feedback.getByText("Đạt")).toBeInTheDocument();
    expect(feedback.getByText("85/100")).toBeInTheDocument();
    expect(feedback.getByText("(85%)")).toBeInTheDocument();
    expect(feedback.getByRole("row", { name: "Truy vấn đúng kết quả 40/40 —" })).toBeInTheDocument();
    expect(screen.getByText("Bài đã được chấm Đạt nên không cần nộp lại.")).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /Nộp/ })).toBeNull();
    expect(screen.queryByLabelText("Đường dẫn repo")).toBeNull();
  });

  it("explains why an ended class takes no more submissions", async () => {
    await renderHomework([
      withSubmissions({
        ...fixtures.mySubmissionsRework,
        canSubmit: false,
        lockedReason: "Lớp đã kết thúc, không nộp bài được nữa.",
      }),
    ]);

    expect(await screen.findByText("Lớp đã kết thúc, không nộp bài được nữa.")).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /Nộp/ })).toBeNull();
    const feedback = within(screen.getByRole("heading", { name: "Kết quả lần nộp #1" }).closest("[data-slot=card]") as HTMLElement);
    expect(feedback.getByText("Cần làm lại")).toBeInTheDocument();
  });

  it("puts the server's field message under its field", async () => {
    const { user } = await renderHomework([
      http.post(submissionsUrl, () =>
        HttpResponse.json(
          {
            error: {
              code: "VALIDATION_FAILED",
              message: "Commit hoặc tag tối đa 100 ký tự, chỉ gồm chữ, số và . _ / -.",
              details: { commitRef: "Commit hoặc tag tối đa 100 ký tự, chỉ gồm chữ, số và . _ / -." },
            },
          },
          { status: 422 },
        ),
      ),
    ]);

    await user.type(await screen.findByLabelText("Đường dẫn repo"), "https://github.com/an/sql-select");
    await user.click(screen.getByRole("button", { name: "Nộp bài" }));

    await waitFor(() => {
      expect(screen.getByLabelText("Commit hoặc tag")).toHaveAccessibleDescription(
        "Không bắt buộc. Giúp giảng viên chấm đúng phiên bản bạn nộp. Commit hoặc tag tối đa 100 ký tự, chỉ gồm chữ, số và . _ / -.",
      );
    });
    expect(screen.queryByRole("alert")).toBeNull();
  });

  it("shows a refusal in the form and reloads the submissions", async () => {
    let gets = 0;
    const { user } = await renderHomework([
      http.get(submissionsUrl, () => {
        gets += 1;
        return HttpResponse.json(fixtures.mySubmissionsRework);
      }),
      http.post(submissionsUrl, () => apiError(409, "INVALID_TRANSITION", "Lớp đã kết thúc, không nộp bài được nữa.")),
    ]);

    await user.click(await screen.findByRole("button", { name: "Nộp lại" }));

    expect(await screen.findByRole("alert")).toHaveTextContent("Lớp đã kết thúc, không nộp bài được nữa.");
    await waitFor(() => {
      expect(gets).toBe(2);
    });
  });
});
