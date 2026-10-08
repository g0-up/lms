import { describe, expect, it } from "vitest";
import { fixtures, ids } from "../test/golden";
import {
  addStageOptions,
  courseDeleteBlocker,
  coursePublishBlocker,
  publishedVersionId,
  removeAt,
  replaceAt,
  requiredTotalLabel,
  stageVersionStatuses,
} from "./publish-blocker";

const statusOf = stageVersionStatuses(fixtures.stageList);

describe("coursePublishBlocker", () => {
  it("needs at least one stage", () => {
    expect(coursePublishBlocker(fixtures.courseVersionDraftEmpty, statusOf)).toBe("Khóa học cần ít nhất một chặng.");
  });

  it("names every attached stage version that is not published", () => {
    expect(coursePublishBlocker(fixtures.courseVersionDraftUnpublishedStage, statusOf)).toBe(
      "Chặng chưa phát hành: Database v2.",
    );
  });

  it("lets a draft of published stage versions through", () => {
    expect(coursePublishBlocker(fixtures.courseVersionDraft, statusOf)).toBeNull();
  });

  it("leaves stage versions it cannot resolve to the server", () => {
    expect(coursePublishBlocker(fixtures.courseVersionDraftUnpublishedStage, stageVersionStatuses(undefined))).toBeNull();
  });

  it("never blocks a version that is not a draft", () => {
    expect(coursePublishBlocker({ ...fixtures.courseVersion, stages: [] }, statusOf)).toBeNull();
  });
});

describe("courseDeleteBlocker", () => {
  it("refuses a published version with classes attached", () => {
    expect(courseDeleteBlocker(fixtures.courseVersion)).toBe("Đang được dùng bởi lớp basic01. Hãy lưu trữ thay vì xóa.");
  });

  it("allows a draft or a version without classes", () => {
    expect(courseDeleteBlocker(fixtures.courseVersionDraft)).toBeNull();
    expect(courseDeleteBlocker({ ...fixtures.courseVersion, classes: [] })).toBeNull();
  });
});

describe("requiredTotalLabel", () => {
  it("sums the required lessons of every stage", () => {
    expect(requiredTotalLabel([{ requiredCount: 1 }, { requiredCount: 3 }])).toBe("4 học liệu bắt buộc toàn khóa");
    expect(requiredTotalLabel([])).toBe("0 học liệu bắt buộc toàn khóa");
  });
});

describe("addStageOptions", () => {
  it("lists published versions of unused stages, marking the newest", () => {
    expect(addStageOptions(fixtures.stageListV2Published, [])).toEqual([
      { value: ids.databaseV1, label: "Database v1" },
      { value: ids.databaseV2, label: "Database v2 (mới nhất)" },
    ]);
  });

  it("skips drafts and stages already in the course version", () => {
    expect(addStageOptions(fixtures.stageList, [])).toEqual([{ value: ids.databaseV1, label: "Database v1 (mới nhất)" }]);
    expect(addStageOptions(fixtures.stageList, fixtures.courseVersionDraft.stages)).toEqual([]);
  });
});

describe("stage list edits", () => {
  const stages = fixtures.stageRows;

  it("finds a published version by number only", () => {
    expect(publishedVersionId(fixtures.stageListV2Published, ids.database, 2)).toBe(ids.databaseV2);
    expect(publishedVersionId(fixtures.stageList, ids.database, 2)).toBeUndefined();
    expect(publishedVersionId(fixtures.stageList, "missing", 1)).toBeUndefined();
  });

  it("replaces and removes by position", () => {
    expect(replaceAt(stages, 0, "new")).toEqual(["new", ids.databaseV2]);
    expect(removeAt(stages, 1)).toEqual([ids.databaseV1]);
  });
});
