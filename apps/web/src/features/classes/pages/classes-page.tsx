import { useQueryClient } from "@tanstack/react-query";
import { Plus } from "lucide-react";
import { useState } from "react";
import { toast } from "sonner";
import { useDocumentTitle } from "@/shared/hooks/use-document-title";
import { Alert, AlertActions } from "@/shared/ui/alert";
import { Button } from "@/shared/ui/button";
import { Card } from "@/shared/ui/card";
import { EmptyState } from "@/shared/ui/empty-state";
import { PageHead } from "@/shared/ui/page-head";
import { Skeleton } from "@/shared/ui/skeleton";
import { TableWrap } from "@/shared/ui/table-wrap";
import { ClassTable } from "../components/class-table";
import { NewClassDialog } from "../components/new-class-dialog";
import { toastFailure } from "../hooks/use-class-mutations";
import { useClassProgress, useClasses } from "../hooks/use-classes";
import { courseVersionsQuery } from "../hooks/use-published-course-versions";
import { teachersQuery } from "../hooks/use-teachers";
import { NO_PUBLISHED_VERSION, publishedVersions } from "../model/class-form";

export function Component() {
  useDocumentTitle("Lớp học");
  const queryClient = useQueryClient();
  const { data, error, isPending, refetch } = useClasses();
  const progress = useClassProgress();
  const [open, setOpen] = useState(false);
  const [opening, setOpening] = useState(false);

  // A class needs a published course version: check before opening, not after filling the form.
  const openNew = async () => {
    setOpening(true);
    try {
      const [courses] = await Promise.all([
        queryClient.query(courseVersionsQuery),
        // Only warms the teacher select: the dialog shows its own loading state if this fails.
        queryClient.query(teachersQuery).catch(() => undefined),
      ]);
      if (publishedVersions(courses).length === 0) toast.error(NO_PUBLISHED_VERSION);
      else setOpen(true);
    } catch (err) {
      if (err instanceof Error) toastFailure(err);
    } finally {
      setOpening(false);
    }
  };

  let body;
  if (isPending) {
    body = (
      <div className="grid gap-3 p-6" aria-busy="true">
        <span className="sr-only">Đang tải danh sách lớp</span>
        {Array.from({ length: 4 }, (_, i) => (
          <Skeleton key={i} className="h-10 w-full" />
        ))}
      </div>
    );
  } else if (error) {
    body = (
      <div className="p-6">
        <Alert variant="danger" role="alert">
          Không tải được danh sách lớp.
          <AlertActions>
            <Button variant="outline" size="sm" onClick={() => void refetch()}>
              Thử lại
            </Button>
          </AlertActions>
        </Alert>
      </div>
    );
  } else {
    body = (
      <TableWrap>
        <ClassTable
          classes={data}
          progress={progress.data}
          empty={
            <EmptyState
              title="Chưa có lớp"
              text="Tạo lớp để mời học viên."
              action={
                <Button variant="outline" size="sm" disabled={opening} onClick={() => void openNew()}>
                  Tạo lớp
                </Button>
              }
            />
          }
        />
      </TableWrap>
    );
  }

  return (
    <>
      <PageHead
        title="Lớp học"
        lede="Mỗi lớp chạy trọn đời trên một phiên bản khóa học. Đổi phiên bản chỉ khi lớp còn ở trạng thái nháp."
        actions={
          <Button variant="gradient" disabled={opening} aria-busy={opening} onClick={() => void openNew()}>
            <Plus aria-hidden="true" />
            Tạo lớp
          </Button>
        }
      />
      <Card>{body}</Card>
      <NewClassDialog open={open} onOpenChange={setOpen} />
    </>
  );
}
