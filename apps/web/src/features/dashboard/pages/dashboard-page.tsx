import { Alert, AlertActions } from "@/shared/ui/alert";
import { Button } from "@/shared/ui/button";
import { Card } from "@/shared/ui/card";
import { PageHead } from "@/shared/ui/page-head";
import { Skeleton } from "@/shared/ui/skeleton";
import { AuditList } from "../components/audit-list";
import { ClassList } from "../components/class-list";
import { kpiGridClass, KpiGrid } from "../components/kpi-grid";
import { OutdatedAlerts } from "../components/outdated-alerts";
import { useDashboard } from "../hooks/use-dashboard";

const cardsClass = "grid grid-cols-2 items-start gap-6 max-[960px]:grid-cols-1";

function DashboardSkeleton() {
  return (
    <div className="grid gap-6" aria-busy="true">
      <span className="sr-only">Đang tải tổng quan</span>
      <div className={kpiGridClass}>
        {[0, 1, 2, 3].map((i) => (
          <Card key={i} className="grid gap-2 px-6 py-4">
            <Skeleton className="h-4 w-2/3" />
            <Skeleton className="h-7 w-1/3" />
            <Skeleton className="h-4 w-1/2" />
          </Card>
        ))}
      </div>
      <div className={cardsClass}>
        {[0, 1].map((i) => (
          <Card key={i} className="grid gap-3 p-6">
            <Skeleton className="h-5 w-1/3" />
            <Skeleton className="h-5 w-full" />
            <Skeleton className="h-5 w-full" />
            <Skeleton className="h-5 w-2/3" />
          </Card>
        ))}
      </div>
    </div>
  );
}

export function Component() {
  const { data, isPending, isError, refetch } = useDashboard();

  let body;
  if (isPending) {
    body = <DashboardSkeleton />;
  } else if (isError) {
    body = (
      <Alert variant="danger" role="alert">
        Không tải được tổng quan.
        <AlertActions>
          <Button variant="outline" size="sm" onClick={() => void refetch()}>
            Thử lại
          </Button>
        </AlertActions>
      </Alert>
    );
  } else {
    body = (
      <div className="grid gap-6">
        <OutdatedAlerts outdated={data.outdated} failedInvites={data.hints.failedInvites} />
        <KpiGrid kpis={data.kpis} hints={data.hints} />
        <div className={cardsClass}>
          <ClassList classes={data.classes} />
          <AuditList activity={data.recentActivity} />
        </div>
      </div>
    );
  }

  return (
    <>
      <PageHead title="Tổng quan" lede="Tình trạng các lớp đang chạy và việc cần xử lý." />
      {body}
    </>
  );
}
