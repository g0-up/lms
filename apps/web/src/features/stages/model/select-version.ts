import type { VersionStatus } from "@/shared/domain";

/** The fields every stage or course version shares. */
export interface VersionLike {
  id: string;
  versionNo: number;
  status: VersionStatus;
}

const byNo = (a: VersionLike, b: VersionLike) => a.versionNo - b.versionNo;

/** Oldest first, as the version lineage reads (the API lists newest first). */
export function sortVersions<T extends VersionLike>(versions: readonly T[]): T[] {
  return [...versions].sort(byNo);
}

export function latestPublished<T extends VersionLike>(versions: readonly T[]): T | undefined {
  return sortVersions(versions.filter((v) => v.status === "published")).at(-1);
}

export function draftOf<T extends VersionLike>(versions: readonly T[]): T | undefined {
  return versions.find((v) => v.status === "draft");
}

/**
 * Version shown on a detail page: the `?v` one when it belongs to this stage/course, else the
 * latest published, else the newest. An unknown `?v` falls back silently.
 */
export function selectVersion<T extends VersionLike>(versions: readonly T[], vParam: string | null): T | undefined {
  const requested = vParam ? versions.find((v) => v.id === vParam) : undefined;
  return requested ?? latestPublished(versions) ?? sortVersions(versions).at(-1);
}

/** Shape `VersionPill`/`Lineage` take. */
export function toVersionRef(v: VersionLike): { id: string; no: number; status: VersionStatus } {
  return { id: v.id, no: v.versionNo, status: v.status };
}
