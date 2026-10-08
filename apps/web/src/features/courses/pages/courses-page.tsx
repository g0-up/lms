import { Plus } from "lucide-react";
import { useState } from "react";
import { useNavigate } from "react-router";
import { toast } from "sonner";
import { Alert, AlertActions } from "@/shared/ui/alert";
import { Button } from "@/shared/ui/button";
import { Card } from "@/shared/ui/card";
import { EmptyState } from "@/shared/ui/empty-state";
import { PageHead } from "@/shared/ui/page-head";
import { Skeleton } from "@/shared/ui/skeleton";
import { CourseTable } from "../components/course-table";
import { NewCourseDialog } from "../components/new-course-dialog";
import { coursePath, useCreateCourse } from "../hooks/use-course-mutations";
import { useCourses } from "../hooks/use-courses";

const LEDE =
  "Khóa học là một chuỗi phiên bản chặng có thứ tự. Lớp gắn với đúng một phiên bản khóa học đã phát hành.";

export function Component() {
  const { data, isPending, isError, refetch } = useCourses();
  const create = useCreateCourse();
  const navigate = useNavigate();
  const [creating, setCreating] = useState(false);

  const openCreate = () => {
    setCreating(true);
  };

  let body;
  if (isPending) {
    body = (
      <Card className="grid gap-3 p-6" aria-busy="true">
        <span className="sr-only">Đang tải danh sách khóa học</span>
        <Skeleton className="h-5 w-1/3" />
        <Skeleton className="h-5 w-full" />
        <Skeleton className="h-5 w-full" />
        <Skeleton className="h-5 w-2/3" />
      </Card>
    );
  } else if (isError) {
    body = (
      <Alert variant="danger" role="alert">
        Không tải được danh sách khóa học.
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
        <EmptyState
          title="Chưa có khóa học"
          text="Tạo khóa học rồi ghép các chặng đã phát hành."
          action={
            <Button onClick={openCreate}>
              <Plus aria-hidden="true" /> Tạo khóa học
            </Button>
          }
        />
      </Card>
    );
  } else {
    body = <CourseTable items={data.items} />;
  }

  return (
    <>
      <PageHead
        title="Khóa học"
        lede={LEDE}
        actions={
          <Button variant="gradient" onClick={openCreate}>
            <Plus aria-hidden="true" /> Tạo khóa học
          </Button>
        }
      />
      {body}
      <NewCourseDialog
        open={creating}
        onOpenChange={setCreating}
        onCreate={async (values) => {
          const course = await create.mutateAsync(values);
          toast.success("Đã tạo khóa học với bản nháp v1.");
          await navigate(coursePath(course.id));
        }}
      />
    </>
  );
}
