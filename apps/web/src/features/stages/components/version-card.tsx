import { fmtDateTime } from "@/shared/lib/format";
import { Card } from "@/shared/ui/card";
import { Lineage } from "@/shared/ui/version-pill";
import { sortVersions, toVersionRef, type VersionLike } from "../model/select-version";

export interface VersionCardVersion extends VersionLike {
  publishedAt?: string;
  clonedFromVersionNo?: number;
}

export interface VersionCardProps {
  versions: readonly VersionCardVersion[];
  current: VersionCardVersion;
  hrefFor: (versionId: string) => string;
}

function versionNote(v: VersionCardVersion): string {
  if (v.status === "published") return `Phát hành ${fmtDateTime(v.publishedAt)}`;
  if (v.status === "archived") return "Đã lưu trữ";
  return v.clonedFromVersionNo ? `Nhân bản từ v${String(v.clonedFromVersionNo)}` : "Bản nháp đầu tiên";
}

/** "Phiên bản" lineage of a stage or course with a note about the version being viewed. */
export function VersionCard({ versions, current, hrefFor }: VersionCardProps) {
  return (
    <Card className="flex flex-wrap items-center justify-between gap-3 p-6 max-[720px]:px-4">
      <div className="flex flex-wrap items-center gap-3">
        <span className="text-xs font-medium tracking-[1px] text-ink-3 uppercase">Phiên bản</span>
        <Lineage
          versions={sortVersions(versions).map(toVersionRef)}
          hrefFor={(v) => hrefFor(v.id)}
          currentId={current.id}
        />
      </div>
      <div className="text-sm text-ink-3">{versionNote(current)}</div>
    </Card>
  );
}
