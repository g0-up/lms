import { Archive, Check, Copy, Pencil, Trash2 } from "lucide-react";
import { Link } from "react-router";
import { Button } from "@/shared/ui/button";
import { PendingLabel } from "@/shared/ui/pending-label";
import { requiredSplitLabel } from "../model/lesson-form";
import type { StageVersion } from "../model/schemas";
import type { VersionLike } from "../model/select-version";
import { stageDeleteBlocker, stagePublishBlocker } from "../model/stage-rules";
import { GuardedButton } from "./guarded-button";

export interface VersionActionsProps {
  /** The stage's draft, if any (a published or archived version links to it instead of cloning). */
  draft: VersionLike | undefined;
  draftHref: string;
  cloning: boolean;
  onPublish: () => void;
  onArchive: () => void;
  onDelete: () => void;
  onClone: () => void;
}

/** Right side of a version's bottom bar; shared by stages and courses, which differ only in their blockers. */
export function VersionActions({
  status,
  versionNo,
  publishBlocker,
  deleteBlocker,
  draft,
  draftHref,
  cloning,
  onPublish,
  onArchive,
  onDelete,
  onClone,
}: VersionActionsProps & Pick<VersionLike, "status" | "versionNo"> & { publishBlocker: string | null; deleteBlocker: string | null }) {
  if (status === "draft") {
    return (
      <div className="flex flex-wrap gap-2">
        <Button variant="outline" onClick={onDelete}>
          <Trash2 aria-hidden="true" /> Xóa bản nháp
        </Button>
        <GuardedButton variant="gradient" blocker={publishBlocker} onClick={onPublish}>
          <Check aria-hidden="true" /> Phát hành v{versionNo}
        </GuardedButton>
      </div>
    );
  }
  return (
    <div className="flex flex-wrap gap-2">
      {status === "published" ? (
        <Button variant="outline" onClick={onArchive}>
          <Archive aria-hidden="true" /> Lưu trữ
        </Button>
      ) : null}
      <GuardedButton variant="outline" blocker={deleteBlocker} onClick={onDelete}>
        <Trash2 aria-hidden="true" /> Xóa
      </GuardedButton>
      {draft ? (
        <Button asChild>
          <Link to={draftHref}>
            <Pencil aria-hidden="true" /> Mở bản nháp v{draft.versionNo}
          </Link>
        </Button>
      ) : (
        <Button disabled={cloning} aria-busy={cloning} onClick={onClone}>
          <Copy aria-hidden="true" />
          <PendingLabel pending={cloning} idle="Nhân bản thành bản nháp" busy="Đang nhân bản…" />
        </Button>
      )}
    </div>
  );
}

/** Bottom bar of the lesson card: required/optional split on the left, version actions on the right. */
export function StageActionsBar({ version, ...actions }: VersionActionsProps & { version: StageVersion }) {
  return (
    <div className="flex flex-wrap items-center justify-between gap-3 border-t border-line px-6 py-4 max-[720px]:px-4">
      <span className="text-sm text-ink-3">{requiredSplitLabel(version.lessons)}</span>
      <VersionActions
        status={version.status}
        versionNo={version.versionNo}
        publishBlocker={stagePublishBlocker(version)}
        deleteBlocker={stageDeleteBlocker(version)}
        {...actions}
      />
    </div>
  );
}
