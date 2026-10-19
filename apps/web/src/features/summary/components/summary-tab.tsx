import { useCallback, useState } from "react";
import { Link, useSearchParams } from "react-router";
import { Alert, AlertActions } from "@/shared/ui/alert";
import { Button } from "@/shared/ui/button";
import { Card } from "@/shared/ui/card";
import { EmptyState } from "@/shared/ui/empty-state";
import { Skeleton } from "@/shared/ui/skeleton";
import { TableWrap } from "@/shared/ui/table-wrap";
import type { StageTarget } from "../api/summary-api";
import { useClassMatrix } from "../hooks/use-class-matrix";
import {
  filterRows,
  hasMatrixFilter,
  memberCount,
  parseMatrixFilter,
  toMatrixQuery,
  withMatrixFilter,
  type MatrixFilter,
} from "../model/matrix-filter";
import { MatrixFilterBar } from "./matrix-filter-bar";
import { StageSummaryDrawer } from "./stage-summary-drawer";
import { MatrixLegend, SummaryMatrix } from "./summary-matrix";

function MatrixSkeleton() {
  return (
    <div className="grid gap-3 px-5 py-4" aria-busy="true">
      <span className="sr-only">Đang tải tổng kết</span>
      {Array.from({ length: 6 }, (_, i) => (
        <Skeleton key={i} className="h-12 w-full" />
      ))}
    </div>
  );
}

export interface SummaryTabProps {
  classId: string;
  /**
   * The tab without filter, e.g. `/admin/classes/{id}?tab=summary`. The filter lives in the current URL next to any
   * other parameter (e.g. `tab`).
   */
  basePath: string;
}

/** FR-82 "Tổng kết" tab shared by admins and teachers: filter bar, member × stage matrix and the stage drawer. */
export function SummaryTab({ classId, basePath }: SummaryTabProps) {
  const [params, setParams] = useSearchParams();
  const { data, error, isPending, refetch } = useClassMatrix(classId);
  const [target, setTarget] = useState<StageTarget | null>(null);
  const closeDrawer = useCallback(() => { setTarget(null); }, []);

  const stageIds = data?.stages.map((s) => s.stageVersionId) ?? [];
  const filter = parseMatrixFilter(params, stageIds);
  const apply = (next: MatrixFilter) => { setParams(withMatrixFilter(params, next)); };

  if (isPending) {
    return (
      <Card>
        <MatrixSkeleton />
      </Card>
    );
  }
  if (error) {
    return (
      <Card className="px-5 py-4">
        <Alert variant="danger" role="alert">
          {error.message}
          <AlertActions>
            <Button variant="outline" size="sm" onClick={() => void refetch()}>
              Thử lại
            </Button>
          </AlertActions>
        </Alert>
      </Card>
    );
  }

  const filtered = hasMatrixFilter(filter);
  const rows = filterRows(data, filter);
  const empty = filtered ? (
    <EmptyState
      title="Không có học viên khớp bộ lọc"
      text="Nới lỏng điều kiện hoặc xóa bộ lọc."
      action={
        <Button asChild variant="outline" size="sm">
          <Link to={basePath}>Xóa bộ lọc</Link>
        </Button>
      }
    />
  ) : (
    <EmptyState title="Chưa có học viên" text="Mời học viên vào lớp để xem tổng kết." />
  );

  return (
    <Card>
      <MatrixFilterBar
        key={toMatrixQuery(filter).toString()}
        filter={filter}
        stages={data.stages}
        clearTo={basePath}
        onApply={apply}
      />
      <div className="flex flex-wrap items-center justify-between gap-3 px-6 pt-4 max-[720px]:px-4">
        <span className="text-sm text-ink-3" aria-live="polite">
          {memberCount(rows.length, data.rows.length)} · Bấm một ô để xem tổng kết chặng và viết nhận xét
        </span>
        <MatrixLegend />
      </div>
      <TableWrap tabIndex={0} role="region" aria-label={`Ma trận tổng kết lớp ${data.class.code}`} className="mt-3">
        <SummaryMatrix matrix={data} rows={rows} onOpen={setTarget} empty={empty} />
      </TableWrap>
      <StageSummaryDrawer classId={classId} target={target} onClose={closeDrawer} />
    </Card>
  );
}
