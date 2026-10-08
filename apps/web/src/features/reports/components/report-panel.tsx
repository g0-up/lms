import { Info } from "lucide-react";
import { useCallback, useState } from "react";
import { Link, useSearchParams } from "react-router";
import { Alert, AlertActions } from "@/shared/ui/alert";
import { Badge } from "@/shared/ui/badge";
import { Button } from "@/shared/ui/button";
import { Card } from "@/shared/ui/card";
import { EmptyState } from "@/shared/ui/empty-state";
import { Skeleton } from "@/shared/ui/skeleton";
import { TableWrap } from "@/shared/ui/table-wrap";
import { useReport } from "../hooks/use-report";
import { hasAnyFilter, parseFilter, toQuery, withFilter, type ReportFilter } from "../model/report-filter";
import type { ReportSort } from "../model/schemas";
import { FilterBar } from "./filter-bar";
import { ReportTable } from "./report-table";
import { StudentDetailDrawer } from "./student-detail-drawer";

function TableSkeleton() {
  return (
    <div className="grid gap-3 px-5 py-4" aria-busy="true">
      <span className="sr-only">Đang tải báo cáo</span>
      {Array.from({ length: 6 }, (_, i) => (
        <Skeleton key={i} className="h-10 w-full" />
      ))}
    </div>
  );
}

export interface ReportPanelProps {
  classId: string;
  /**
   * The report page without filter: `/admin/classes/{id}?tab=report` or `/teach/classes/{id}`.
   * The filter lives in the current URL next to any other parameter (e.g. `tab`).
   */
  basePath: string;
}

/** FR-40 class report shared by admins and teachers: filter bar, table and member drilldown. */
export function ReportPanel({ classId, basePath }: ReportPanelProps) {
  const [params, setParams] = useSearchParams();
  const filter = parseFilter(params);
  const { data, error, isPending, isPlaceholderData, refetch } = useReport(classId, filter);
  const [openMember, setOpenMember] = useState<string | null>(null);
  const closeDrawer = useCallback(() => { setOpenMember(null); }, []);

  const apply = (next: ReportFilter) => { setParams(withFilter(params, next)); };
  const onSort = (sort: ReportSort) => { apply({ ...filter, sort }); };
  const filtered = hasAnyFilter(filter);

  let body;
  if (isPending) {
    body = <TableSkeleton />;
  } else if (error) {
    body = (
      <div className="px-5 py-4">
        <Alert variant="danger" role="alert">
          Không tải được báo cáo.
          <AlertActions>
            <Button variant="outline" size="sm" onClick={() => void refetch()}>
              Thử lại
            </Button>
          </AlertActions>
        </Alert>
      </div>
    );
  } else {
    // Dropped rows are shown on request but, like the total, never counted.
    const counted = data.rows.filter((r) => r.memberStatus !== "dropped").length;
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
      <EmptyState title="Chưa có học viên" text="Mời học viên vào lớp để theo dõi tiến độ." />
    );
    body = (
      <>
        <div className="flex flex-wrap items-center justify-between gap-3 px-6 pt-4 max-[720px]:px-4">
          <span className="text-sm text-ink-3">
            {counted}/{data.summary.activeCount} học viên · Bấm vào một dòng để xem từng học liệu
          </span>
          <Badge variant="warn">
            <Info aria-hidden="true" className="size-3.5" />
            Tiến độ do học viên tự xác nhận
          </Badge>
        </div>
        <TableWrap>
          <ReportTable
            report={data}
            sort={filter.sort}
            onSort={onSort}
            onOpen={setOpenMember}
            empty={empty}
            busy={isPlaceholderData}
          />
        </TableWrap>
      </>
    );
  }

  return (
    <Card>
      <FilterBar key={toQuery(filter).toString()} filter={filter} clearTo={basePath} onApply={apply} />
      {body}
      <StudentDetailDrawer classId={classId} memberId={openMember} onClose={closeDrawer} />
    </Card>
  );
}
