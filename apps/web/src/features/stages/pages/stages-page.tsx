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
import { NewStageDialog } from "../components/new-stage-dialog";
import { StageTable } from "../components/stage-table";
import { stagePath, useCreateStage } from "../hooks/use-stage-mutations";
import { useStages } from "../hooks/use-stages";

const LEDE =
  "Chặng dùng chung giữa các khóa học. Sửa học liệu bằng cách nhân bản ra bản nháp mới; phiên bản đã phát hành không đổi.";

export function Component() {
  const { data, isPending, isError, refetch } = useStages();
  const create = useCreateStage();
  const navigate = useNavigate();
  const [creating, setCreating] = useState(false);

  const openCreate = () => {
    setCreating(true);
  };

  let body;
  if (isPending) {
    body = (
      <Card className="grid gap-3 p-6" aria-busy="true">
        <span className="sr-only">Đang tải danh sách chặng</span>
        <Skeleton className="h-5 w-1/3" />
        <Skeleton className="h-5 w-full" />
        <Skeleton className="h-5 w-full" />
        <Skeleton className="h-5 w-2/3" />
      </Card>
    );
  } else if (isError) {
    body = (
      <Alert variant="danger" role="alert">
        Không tải được danh sách chặng.
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
          title="Chưa có chặng"
          text="Tạo chặng đầu tiên để bắt đầu soạn học liệu."
          action={
            <Button onClick={openCreate}>
              <Plus aria-hidden="true" /> Tạo chặng
            </Button>
          }
        />
      </Card>
    );
  } else {
    body = <StageTable items={data.items} />;
  }

  return (
    <>
      <PageHead
        title="Chặng"
        lede={LEDE}
        actions={
          <Button variant="gradient" onClick={openCreate}>
            <Plus aria-hidden="true" /> Tạo chặng
          </Button>
        }
      />
      {body}
      <NewStageDialog
        open={creating}
        onOpenChange={setCreating}
        onCreate={async (values) => {
          const stage = await create.mutateAsync(values);
          toast.success("Đã tạo chặng với bản nháp v1.");
          await navigate(stagePath(stage.id));
        }}
      />
    </>
  );
}
