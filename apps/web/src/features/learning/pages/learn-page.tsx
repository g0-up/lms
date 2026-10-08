import { useMe } from "@/features/auth";
import { Alert, AlertActions } from "@/shared/ui/alert";
import { Button } from "@/shared/ui/button";
import { Card } from "@/shared/ui/card";
import { EmptyState } from "@/shared/ui/empty-state";
import { PageHead } from "@/shared/ui/page-head";
import { Skeleton } from "@/shared/ui/skeleton";
import { ClassCard } from "../components/class-card";
import { useMyClasses } from "../hooks/use-my-classes";

const gridClass = "grid grid-cols-2 items-start gap-6 max-[720px]:grid-cols-1";

function ClassCardSkeleton() {
  return (
    <Card className="grid gap-3 p-6 max-[720px]:px-4">
      <Skeleton className="h-6 w-2/3" />
      <Skeleton className="h-4 w-1/2" />
      <Skeleton className="h-2.5 w-full" />
      <Skeleton className="h-11 w-40 rounded-full" />
    </Card>
  );
}

/** Last word of a Vietnamese full name, the one people are called by ("Nguyễn Hoàng An" → "An"). */
const givenName = (fullName: string) => fullName.trim().split(/\s+/).pop() ?? "";

export function Component() {
  const me = useMe().data;
  const { data, isPending, isError, refetch } = useMyClasses();
  const primaryId = data?.items.find((c) => c.status === "active")?.id;

  let body;
  if (isPending) {
    body = (
      <div className={gridClass} aria-busy="true">
        <span className="sr-only">Đang tải danh sách lớp</span>
        <ClassCardSkeleton />
        <ClassCardSkeleton />
      </div>
    );
  } else if (isError) {
    body = (
      <Alert variant="danger" role="alert">
        Không tải được danh sách lớp.
        <AlertActions>
          <Button variant="outline" size="sm" onClick={() => void refetch()}>
            Thử lại
          </Button>
        </AlertActions>
      </Alert>
    );
  } else if (data.items.length === 0) {
    body = (
      <Card>
        <EmptyState title="Chưa có lớp" text="Khi được mời vào lớp, lớp sẽ hiện ở đây." />
      </Card>
    );
  } else {
    body = (
      <div className={gridClass}>
        {data.items.map((item) => (
          <ClassCard key={item.id} item={item} primary={item.id === primaryId} />
        ))}
      </div>
    );
  }

  return (
    <>
      <PageHead title={`Xin chào, ${givenName(me?.name ?? "")}`} lede="Các lớp bạn đang tham gia." />
      {body}
    </>
  );
}
