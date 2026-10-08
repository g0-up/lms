import { TriangleAlert, X } from "lucide-react";
import { ItemBody, ItemMeta, ItemTitleLink, List, ListItem, Ord, OrderButtons, type VersionLike } from "@/features/stages";
import { STATUS_VI } from "@/shared/domain";
import { cn } from "@/shared/lib/cn";
import { Badge } from "@/shared/ui/badge";
import { Button } from "@/shared/ui/button";
import { EmptyState } from "@/shared/ui/empty-state";
import { IconButton } from "@/shared/ui/icon-button";
import type { CourseStage } from "../model/schemas";

type VersionStatus = VersionLike["status"];

const TAG_CLASS: Record<VersionStatus, string> = {
  published: "bg-navy-700 text-white",
  draft: "border-dashed",
  archived: "border-gray-300 bg-surface-100 text-gray-600",
};

/** Small, non-interactive version pill for meta lines. */
function VersionTag({ no, status }: { no: number; status: VersionStatus }) {
  return (
    <span
      title={STATUS_VI[status]}
      className={cn(
        "inline-flex h-[22px] items-center rounded-pill border border-navy-700 bg-white px-2 text-xs font-semibold text-navy-700",
        TAG_CLASS[status],
      )}
    >
      v{no}
    </span>
  );
}

export interface CourseStageListProps {
  stages: readonly CourseStage[];
  /** Only a draft shows the swap, reorder and remove controls. */
  editable: boolean;
  /** Stage version status from the stage list; the course version itself does not carry it. */
  statusOf: (stageVersionId: string) => VersionStatus | undefined;
  onSwap: (index: number) => void;
  onMove: (index: number, delta: -1 | 1) => void;
  onRemove: (index: number) => void;
}

export function CourseStageList({ stages, editable, statusOf, onSwap, onMove, onRemove }: CourseStageListProps) {
  if (stages.length === 0) {
    return (
      <EmptyState
        title="Chưa có chặng"
        text={editable ? "Thêm các chặng đã phát hành theo thứ tự học." : "Phiên bản này không có chặng."}
      />
    );
  }
  return (
    <List>
      {stages.map((stage, i) => (
        <ListItem key={stage.stageVersionId}>
          <Ord n={i + 1} />
          <ItemBody>
            <ItemTitleLink to={`/admin/stages/${stage.stageId}?v=${stage.stageVersionId}`}>{stage.stageName}</ItemTitleLink>
            <ItemMeta>
              <VersionTag no={stage.stageVersionNo} status={statusOf(stage.stageVersionId) ?? "published"} />
              <span>
                {stage.lessonCount} học liệu · {stage.requiredCount} bắt buộc
              </span>
              {stage.outdated && stage.latestPublishedNo ? (
                <Badge variant="warn">
                  <TriangleAlert aria-hidden="true" className="size-3" /> v{stage.latestPublishedNo} đã phát hành
                </Badge>
              ) : null}
            </ItemMeta>
          </ItemBody>
          {editable ? (
            <div className="flex shrink-0 flex-wrap items-center justify-end gap-0.5">
              {stage.outdated && stage.latestPublishedNo ? (
                <Button
                  variant="ghost"
                  size="sm"
                  onClick={() => {
                    onSwap(i);
                  }}
                >
                  Dùng v{stage.latestPublishedNo}
                </Button>
              ) : null}
              <OrderButtons
                index={i}
                count={stages.length}
                onMove={(delta) => {
                  onMove(i, delta);
                }}
              />
              <IconButton
                aria-label="Gỡ khỏi khóa học"
                onClick={() => {
                  onRemove(i);
                }}
              >
                <X aria-hidden="true" />
              </IconButton>
            </div>
          ) : null}
        </ListItem>
      ))}
    </List>
  );
}
