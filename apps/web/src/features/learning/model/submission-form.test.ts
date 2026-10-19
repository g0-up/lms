import { describe, expect, it } from "vitest";
import { ApiError } from "@/shared/api/errors";
import {
  SUBMISSION_MESSAGES,
  fileProblem,
  isValidRepoUrl,
  submissionFieldErrors,
  submissionFormSchema,
  submittedMessage,
  toSubmitBody,
} from "./submission-form";

const MB = 1_048_576;
const file = (name: string, size = 10) => {
  const f = new File(["x"], name);
  Object.defineProperty(f, "size", { value: size });
  return f;
};
const schema = submissionFormSchema(5);
const errorsOf = (input: Parameters<typeof schema.safeParse>[0]) => {
  const result = schema.safeParse(input);
  return result.success ? {} : Object.fromEntries(result.error.issues.map((i) => [String(i.path[0]), i.message]));
};

describe("isValidRepoUrl", () => {
  it("accepts https repos on GitHub, GitLab and Bitbucket, with or without a deeper path", () => {
    expect(isValidRepoUrl("https://github.com/an/sql-select")).toBe(true);
    expect(isValidRepoUrl("https://www.GitHub.com/an/sql-select/tree/main")).toBe(true);
    expect(isValidRepoUrl("https://gitlab.com/an/repo")).toBe(true);
    expect(isValidRepoUrl("https://bitbucket.org/an/repo")).toBe(true);
  });

  it("rejects other hosts, plain http, a missing repo and an upper-case scheme", () => {
    expect(isValidRepoUrl("https://example.com/an/repo")).toBe(false);
    expect(isValidRepoUrl("http://github.com/an/repo")).toBe(false);
    expect(isValidRepoUrl("https://github.com/an")).toBe(false);
    expect(isValidRepoUrl("HTTPS://github.com/an/repo")).toBe(false);
    expect(isValidRepoUrl(`https://github.com/an/${"r".repeat(500)}`)).toBe(false);
  });
});

describe("submissionFormSchema", () => {
  it("accepts a repo alone, files alone, or both, trimming the text fields", () => {
    expect(schema.parse({ repoUrl: " https://github.com/an/repo ", commitRef: " v1.0 ", files: [] })).toEqual({
      repoUrl: "https://github.com/an/repo",
      commitRef: "v1.0",
      files: [],
    });
    expect(schema.safeParse({ repoUrl: "", commitRef: "", files: [file("bai.zip")] }).success).toBe(true);
    expect(
      schema.safeParse({ repoUrl: "https://github.com/an/repo", commitRef: "a1b2c3d", files: [file("a.md")] }).success,
    ).toBe(true);
  });

  it("needs a repo or at least one file", () => {
    expect(errorsOf({ repoUrl: "  ", commitRef: "", files: [] })).toEqual({ repoUrl: SUBMISSION_MESSAGES.noContent });
  });

  it("explains the repo rule", () => {
    expect(errorsOf({ repoUrl: "https://example.com/a/b", commitRef: "", files: [] })).toEqual({
      repoUrl: SUBMISSION_MESSAGES.repoUrl,
    });
  });

  it("only takes a commit with a repo, and only a short ref of safe characters", () => {
    expect(errorsOf({ repoUrl: "", commitRef: "abc", files: [file("a.pdf")] })).toEqual({
      commitRef: SUBMISSION_MESSAGES.commitNeedsRepo,
    });
    const repo = "https://github.com/an/repo";
    expect(errorsOf({ repoUrl: repo, commitRef: "a b", files: [] })).toEqual({ commitRef: SUBMISSION_MESSAGES.commitRef });
    expect(errorsOf({ repoUrl: repo, commitRef: "a".repeat(101), files: [] })).toEqual({
      commitRef: SUBMISSION_MESSAGES.commitRef,
    });
    expect(schema.safeParse({ repoUrl: repo, commitRef: "a".repeat(100), files: [] }).success).toBe(true);
    expect(schema.safeParse({ repoUrl: repo, commitRef: "release/v1.0_rc-2", files: [] }).success).toBe(true);
  });

  it("takes at most the API's number of files", () => {
    const five = ["1.md", "2.md", "3.md", "4.md", "5.md"].map((n) => file(n));
    expect(schema.safeParse({ repoUrl: "", commitRef: "", files: five }).success).toBe(true);
    expect(errorsOf({ repoUrl: "", commitRef: "", files: [...five, file("6.md")] })).toEqual({
      files: SUBMISSION_MESSAGES.tooManyFiles(5),
    });
    expect(errorsOf({ repoUrl: "", commitRef: "", files: five.slice(0, 3) })).toEqual({});
    expect(
      submissionFormSchema(2).safeParse({ repoUrl: "", commitRef: "", files: five.slice(0, 3) }).error?.issues[0]
        .message,
    ).toBe("Tối đa 2 file mỗi lần nộp.");
  });

  it("names the first file of a wrong type or over 20 MB", () => {
    expect(errorsOf({ repoUrl: "", commitRef: "", files: [file("a.md"), file("setup.exe")] })).toEqual({
      files: "File setup.exe không đúng định dạng. Chấp nhận: zip, pdf, md, txt, sql, png, jpg, jpeg.",
    });
    expect(errorsOf({ repoUrl: "", commitRef: "", files: [file("video.mp4")] }).files).toMatch(/^File video\.mp4/);
    expect(errorsOf({ repoUrl: "", commitRef: "", files: [file("big.zip", 20 * MB + 1)] })).toEqual({
      files: "File big.zip lớn hơn 20 MB.",
    });
  });
});

describe("fileProblem", () => {
  it("accepts every allowed extension in any case, up to exactly 20 MB", () => {
    for (const name of ["a.zip", "a.PDF", "a.md", "a.txt", "a.sql", "a.png", "a.JPG", "a.jpeg"]) {
      expect(fileProblem({ name, size: 20 * MB })).toBeNull();
    }
  });

  it("rejects a file without an extension", () => {
    expect(fileProblem({ name: "Makefile", size: 1 })).toBe(SUBMISSION_MESSAGES.badType("Makefile"));
  });
});

describe("toSubmitBody / submittedMessage", () => {
  it("sends the uploaded media ids with the text fields", () => {
    expect(toSubmitBody({ repoUrl: "https://github.com/an/repo", commitRef: "" }, ["m1"])).toEqual({
      repoUrl: "https://github.com/an/repo",
      commitRef: "",
      mediaIds: ["m1"],
    });
  });

  it("says which submission was recorded, whether it was late and what it replaced", () => {
    expect(submittedMessage({ no: 1, isLate: false, replaced: null })).toBe(
      "Đã nộp lần #1. Bạn sẽ nhận email khi có kết quả.",
    );
    expect(submittedMessage({ no: 3, isLate: true, replaced: 2 })).toBe(
      "Đã nộp lần #3 (nộp muộn), thay lần #2 đang chờ chấm. Bạn sẽ nhận email khi có kết quả.",
    );
  });
});

describe("submissionFieldErrors", () => {
  it("maps the API's per-field messages to the form fields", () => {
    const error = new ApiError(422, "VALIDATION_FAILED", "Commit…", {
      commitRef: "Commit hoặc tag chỉ đi kèm đường dẫn repo.",
      other: "x",
    });
    expect(submissionFieldErrors(error)).toEqual({ commitRef: "Commit hoặc tag chỉ đi kèm đường dẫn repo." });
  });

  it("returns nothing for other errors or details it cannot read", () => {
    expect(submissionFieldErrors(new ApiError(409, "INVALID_TRANSITION", "x", { repoUrl: "y" }))).toEqual({});
    expect(submissionFieldErrors(new ApiError(422, "VALIDATION_FAILED", "x"))).toEqual({});
    expect(submissionFieldErrors(new Error("boom"))).toEqual({});
  });
});
