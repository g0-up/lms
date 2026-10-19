import { Link, useSearchParams } from "react-router";
import { GRADING_SLA_DAYS } from "@/shared/domain";
import { Alert, AlertActions } from "@/shared/ui/alert";
import { Badge } from "@/shared/ui/badge";
import { Button } from "@/shared/ui/button";
import { Card, CardHeader, CardTitle } from "@/shared/ui/card";
import { EmptyState } from "@/shared/ui/empty-state";
import { PageHead } from "@/shared/ui/page-head";
import { Skeleton } from "@/shared/ui/skeleton";
import { TableWrap } from "@/shared/ui/table-wrap";
import { QueueFilterBar } from "../components/queue-filter-bar";
import { QueueTable } from "../components/queue-table";
import { RecentList } from "../components/recent-list";
import { useGradingBase } from "../hooks/use-grading-base";
import { useQueue, useRecent } from "../hooks/use-queue";
import { queueHref } from "../model/links";
import { hasAnyQueueFilter, parseQueueFilter, toQueueQuery, type QueueFilter } from "../model/queue-filter";

function ListSkeleton({ label, rows }: { label: string; rows: number }) {
  return (
    <div className="grid gap-3 px-5 py-4" aria-busy="true">
      <span className="sr-only">{label}</span>
      {Array.from({ length: rows }, (_, i) => (
        <Skeleton key={i} className="h-10 w-full" />
      ))}
    </div>
  );
}

function LoadError({ text, onRetry }: { text: string; onRetry: () => void }) {
  return (
    <div className="px-5 py-4">
      <Alert variant="danger" role="alert">
        {text}
        <AlertActions>
          <Button variant="outline" size="sm" onClick={onRetry}>
            Thử lại
          </Button>
        </AlertActions>
      </Alert>
    </div>
  );
}

function QueueSection({ filter, onApply }: { filter: QueueFilter; onApply: (filter: QueueFilter) => void }) {
  const base = useGradingBase();
  const { data, error, isPending, isPlaceholderData, refetch } = useQueue(filter);
  if (isPending) return <ListSkeleton label="Đang tải hàng chờ" rows={5} />;
  if (error) return <LoadError text="Không tải được hàng chờ." onRetry={() => void refetch()} />;

  const empty = hasAnyQueueFilter(filter) ? (
    <EmptyState
      title="Không có mục khớp bộ lọc"
      text="Nới lỏng điều kiện hoặc xóa bộ lọc."
      action={
        <Button asChild variant="outline" size="sm">
          <Link to={queueHref(base)}>Xóa bộ lọc</Link>
        </Button>
      }
    />
  ) : (
    <EmptyState
      title="Không còn bài chờ xử lý"
      text="Bài tập học viên nộp và lượt Quiz bài tập đã nộp hiện ở đây, cũ nhất ở trên."
    />
  );
  return (
    <>
      <QueueFilterBar
        key={toQueueQuery(filter).toString()}
        filter={filter}
        options={data.filters}
        clearTo={queueHref(base)}
        onApply={onApply}
      />
      <p className="px-6 pt-4 text-sm text-ink-3 max-[720px]:px-4">
        {data.items.length}/{data.total} mục chờ xử lý · {data.quizPending} lượt quiz chờ xác nhận
      </p>
      <TableWrap>
        <QueueTable items={data.items} base={base} empty={empty} busy={isPlaceholderData} />
      </TableWrap>
    </>
  );
}

function RecentSection() {
  const base = useGradingBase();
  const { data, error, isPending, refetch } = useRecent();
  if (isPending) return <ListSkeleton label="Đang tải bài đã chấm gần đây" rows={3} />;
  if (error) return <LoadError text="Không tải được bài đã chấm gần đây." onRetry={() => void refetch()} />;
  return <RecentList items={data} base={base} />;
}

/** Overdue badge of the whole queue; the count does not depend on the filter. */
function OverdueBadge({ filter }: { filter: QueueFilter }) {
  const { data } = useQueue(filter);
  if (!data?.overdueCount) return null;
  return (
    <Badge variant="danger">
      {data.overdueCount} mục quá {GRADING_SLA_DAYS} ngày
    </Badge>
  );
}

/** FR-63 grading queue of a teacher (their classes) or an admin (every class), with recent gradings. */
export function Component() {
  const [params, setParams] = useSearchParams();
  const filter = parseQueueFilter(params);

  return (
    <>
      <PageHead
        title="Chấm bài"
        lede={`Bài tập chờ chấm và lượt Quiz bài tập chờ xác nhận, cũ nhất ở trên. Mục tiêu xử lý trong ${String(GRADING_SLA_DAYS)} ngày kể từ lúc nộp.`}
        badges={<OverdueBadge filter={filter} />}
      />
      <div className="grid gap-6">
        <section aria-label="Hàng chờ chấm">
          <Card>
            <QueueSection filter={filter} onApply={(next) => { setParams(toQueueQuery(next)); }} />
          </Card>
        </section>
        <Card>
          <CardHeader>
            <CardTitle>Đã chấm và xác nhận gần đây</CardTitle>
          </CardHeader>
          <RecentSection />
        </Card>
      </div>
    </>
  );
}
