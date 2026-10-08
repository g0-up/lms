import { Alert, AlertActions } from "@/shared/ui/alert";
import { Button } from "@/shared/ui/button";
import { Card } from "@/shared/ui/card";
import { EmptyState } from "@/shared/ui/empty-state";
import { PageHead } from "@/shared/ui/page-head";
import { Skeleton } from "@/shared/ui/skeleton";
import { TeacherClassCard } from "../components/teacher-class-card";
import { useTeachClasses } from "../hooks/use-teach-classes";

const gridClass = "grid grid-cols-3 items-start gap-6 max-[960px]:grid-cols-2 max-[720px]:grid-cols-1";

function CardSkeleton() {
  return (
    <Card className="grid gap-3 p-6 max-[720px]:px-4">
      <Skeleton className="h-6 w-1/2" />
      <Skeleton className="h-4 w-3/4" />
      <Skeleton className="h-2 w-full" />
      <Skeleton className="h-4 w-1/3" />
    </Card>
  );
}

export function Component() {
  const { data, isPending, isError, refetch } = useTeachClasses();

  let body;
  if (isPending) {
    body = (
      <div className={gridClass} aria-busy="true">
        <span className="sr-only">Đang tải danh sách lớp</span>
        <CardSkeleton />
        <CardSkeleton />
        <CardSkeleton />
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
  } else if (data.length === 0) {
    body = (
      <Card>
        <EmptyState title="Chưa có lớp" text="Bạn chưa được phân công lớp nào." />
      </Card>
    );
  } else {
    body = (
      <div className={gridClass}>
        {data.map((item) => (
          <TeacherClassCard key={item.id} item={item} />
        ))}
      </div>
    );
  }

  return (
    <>
      <PageHead
        title="Lớp của tôi"
        lede="Các lớp bạn phụ trách. Vào lớp để xem tiến độ từng học viên theo chặng."
      />
      {body}
    </>
  );
}
