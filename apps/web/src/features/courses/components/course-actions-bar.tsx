import { VersionActions, type VersionActionsProps } from "@/features/stages";
import { courseDeleteBlocker, requiredTotalLabel } from "../model/publish-blocker";
import type { CourseVersion } from "../model/schemas";

/** Bottom bar of the stage card: required lesson total on the left, version actions on the right. */
export function CourseActionsBar({
  version,
  publishBlocker,
  ...actions
}: VersionActionsProps & { version: CourseVersion; publishBlocker: string | null }) {
  return (
    <div className="flex flex-wrap items-center justify-between gap-3 border-t border-line px-6 py-4 max-[720px]:px-4">
      <span className="text-sm text-ink-3">{requiredTotalLabel(version.stages)}</span>
      <VersionActions
        status={version.status}
        versionNo={version.versionNo}
        publishBlocker={publishBlocker}
        deleteBlocker={courseDeleteBlocker(version)}
        {...actions}
      />
    </div>
  );
}
