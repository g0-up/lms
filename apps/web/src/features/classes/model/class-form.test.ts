import { describe, expect, it } from "vitest";
import type { z } from "zod";
import { ApiError, NETWORK_ERROR_MESSAGE } from "@/shared/api/errors";
import { fixtures } from "../test/golden";
import {
  changedFields,
  classFormError,
  classFormSchema,
  datesError,
  defaultDates,
  defaultVersionId,
  publishedVersions,
  settingsFormSchema,
  settingsVersionOptions,
  type ClassFormInput,
} from "./class-form";
import type { CourseList } from "./schemas";

const valid: ClassFormInput = {
  code: " basic04 ",
  name: "Lập trình cơ bản – khóa 4",
  courseVersionId: "v1",
  startDate: "2026-10-19",
  endDate: "2027-02-02",
  teacherId: "t1",
};

function messages(input: unknown, schema: z.ZodType = classFormSchema) {
  const result = schema.safeParse(input);
  return result.success ? [] : result.error.issues.map((i) => [i.path.join("."), i.message]);
}

describe("datesError", () => {
  it.each([
    ["", "2026-10-20", { field: "startDate", message: "Nhập ngày bắt đầu và kết thúc dự kiến." }],
    ["2026-10-20", "", { field: "endDate", message: "Nhập ngày bắt đầu và kết thúc dự kiến." }],
    ["2026-10-20", "2026-10-20", { field: "endDate", message: "Ngày kết thúc phải sau ngày bắt đầu." }],
    ["2026-10-20", "2026-10-19", { field: "endDate", message: "Ngày kết thúc phải sau ngày bắt đầu." }],
    ["2026-10-20", "2026-10-21", null],
    ["2026-12-31", "2027-01-01", null],
  ])("start %s end %s", (start, end, expected) => {
    expect(datesError(start, end)).toEqual(expected);
  });
});

describe("classFormSchema", () => {
  it("accepts a complete form and trims the code and name", () => {
    expect(classFormSchema.parse(valid)).toMatchObject({ code: "basic04", name: "Lập trình cơ bản – khóa 4" });
  });

  it("asks for the code and name together", () => {
    expect(messages({ ...valid, code: " " })).toEqual([["code", "Nhập mã và tên lớp."]]);
    expect(messages({ ...valid, name: "" })).toEqual([["name", "Nhập mã và tên lớp."]]);
  });

  it("reports the date, teacher and version rules", () => {
    expect(messages({ ...valid, endDate: valid.startDate, teacherId: "", courseVersionId: "" })).toEqual([
      ["courseVersionId", "Chọn một phiên bản khóa học đã phát hành."],
      ["endDate", "Ngày kết thúc phải sau ngày bắt đầu."],
      ["teacherId", "Chọn giảng viên phụ trách."],
    ]);
  });
});

describe("settingsFormSchema", () => {
  it("asks for the class name", () => {
    const { code: _code, ...settings } = valid;
    expect(messages({ ...settings, name: "  " }, settingsFormSchema)).toEqual([["name", "Nhập tên lớp."]]);
  });
});

describe("defaultDates", () => {
  it("starts in 14 days and ends 120 days from today, across month and year ends", () => {
    expect(defaultDates(new Date(2026, 9, 5))).toEqual({ startDate: "2026-10-19", endDate: "2027-02-02" });
  });
});

describe("changedFields", () => {
  it("keeps only the edited fields", () => {
    expect(changedFields({ name: "a", teacherId: "t1" }, { name: "b", teacherId: "t1" })).toEqual({ name: "b" });
  });
});

const courses: CourseList = {
  items: [
    {
      id: "c1",
      code: "BASIC",
      name: "Lập trình cơ bản",
      versions: [
        { id: "b3", versionNo: 3, status: "draft", publishedAt: null },
        { id: "b2", versionNo: 2, status: "published", publishedAt: "2026-09-01T08:00:00Z" },
        { id: "b1", versionNo: 1, status: "archived", publishedAt: "2026-08-01T08:00:00Z" },
      ],
    },
    {
      id: "c2",
      code: "ADV",
      name: "Nâng cao",
      versions: [{ id: "a1", versionNo: 1, status: "published", publishedAt: "2026-09-20T08:00:00Z" }],
    },
  ],
};

describe("published versions", () => {
  it("lists only published versions, labelled with course and number", () => {
    expect(publishedVersions(courses).map((o) => o.label)).toEqual(["Lập trình cơ bản v2", "Nâng cao v1"]);
  });

  it("defaults to the most recently published version", () => {
    expect(defaultVersionId(publishedVersions(courses))).toBe("a1");
    expect(defaultVersionId(publishedVersions(fixtures.courseList))).toBe(fixtures.courseList.items[0]?.versions[0]?.id);
    expect(defaultVersionId([])).toBe("");
  });

  it("adds the class's current version with its status when it is no longer published", () => {
    const options = settingsVersionOptions(courses, { id: "b1", courseName: "Lập trình cơ bản", versionNo: 1 });
    expect(options.map((o) => o.label)).toEqual(["Lập trình cơ bản v1 (Lưu trữ)", "Lập trình cơ bản v2", "Nâng cao v1"]);
  });

  it("does not repeat a current version that is published", () => {
    expect(settingsVersionOptions(courses, { id: "b2", courseName: "Lập trình cơ bản", versionNo: 2 })).toHaveLength(2);
  });
});

describe("classFormError", () => {
  it("puts a taken code on the code field", () => {
    expect(classFormError(new ApiError(409, "CONFLICT", "Mã lớp đã tồn tại."))).toEqual({
      field: "code",
      message: "Mã lớp đã tồn tại.",
    });
  });

  it("shows any other refusal with the server message, and a network failure with the generic copy", () => {
    expect(classFormError(new ApiError(422, "VALIDATION_FAILED", "Ngày kết thúc phải sau ngày bắt đầu."))).toEqual({
      field: null,
      message: "Ngày kết thúc phải sau ngày bắt đầu.",
    });
    expect(classFormError(new TypeError("Failed to fetch"))).toEqual({ field: null, message: NETWORK_ERROR_MESSAGE });
  });
});
