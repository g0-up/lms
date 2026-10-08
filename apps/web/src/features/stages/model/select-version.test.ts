import { describe, expect, it } from "vitest";
import { draftOf, latestPublished, selectVersion, sortVersions, toVersionRef, type VersionLike } from "./select-version";

const v = (versionNo: number, status: VersionLike["status"]): VersionLike => ({ id: `v${String(versionNo)}`, versionNo, status });
// Newest first, as the API lists them.
const versions = [v(4, "draft"), v(3, "published"), v(2, "archived"), v(1, "published")];

describe("version selection", () => {
  it("sorts oldest first without mutating the input", () => {
    expect(sortVersions(versions).map((x) => x.versionNo)).toEqual([1, 2, 3, 4]);
    expect(versions[0].versionNo).toBe(4);
  });

  it("finds the latest published and the draft", () => {
    expect(latestPublished(versions)?.id).toBe("v3");
    expect(draftOf(versions)?.id).toBe("v4");
    expect(draftOf([v(1, "published")])).toBeUndefined();
  });

  it("honours a known ?v", () => {
    expect(selectVersion(versions, "v2")?.id).toBe("v2");
    expect(selectVersion(versions, "v4")?.id).toBe("v4");
  });

  it("falls back to the latest published for a missing or unknown ?v", () => {
    expect(selectVersion(versions, null)?.id).toBe("v3");
    expect(selectVersion(versions, "nope")?.id).toBe("v3");
  });

  it("falls back to the newest version when none is published", () => {
    expect(selectVersion([v(1, "draft")], null)?.id).toBe("v1");
    expect(selectVersion(versions.slice(0, 0), null)).toBeUndefined();
  });

  it("maps to the version pill shape", () => {
    expect(toVersionRef(v(2, "archived"))).toEqual({ id: "v2", no: 2, status: "archived" });
  });
});
