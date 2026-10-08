import { Play } from "lucide-react";
import { useState } from "react";
import { Button } from "@/shared/ui/button";
import { ConfirmDialog } from "@/shared/ui/confirm-dialog";
import { useClassTransition } from "../hooks/use-class-mutations";
import type { ClassDetail } from "../model/schemas";

/** The class's next lifecycle step behind a confirmation: draft → active → ended, nothing after. */
export function LifecycleAction({ cls }: { cls: ClassDetail }) {
  const [open, setOpen] = useState(false);
  const transition = useClassTransition(cls.id);
  if (cls.status === "ended") return null;

  const activate = cls.status === "draft";
  const confirm = () => {
    transition.mutate(activate ? "activate" : "end", { onSettled: () => { setOpen(false); } });
  };

  return (
    <>
      {activate ? (
        <Button variant="gradient" onClick={() => { setOpen(true); }}>
          <Play aria-hidden="true" />
          Kích hoạt lớp
        </Button>
      ) : (
        <Button variant="outline" onClick={() => { setOpen(true); }}>
          Kết thúc lớp
        </Button>
      )}
      {activate ? (
        <ConfirmDialog
          open={open}
          onOpenChange={setOpen}
          title={`Kích hoạt lớp ${cls.code}?`}
          text={`${String(cls.memberCount)} học viên sẽ bắt đầu học và tích hoàn thành được. Sau khi kích hoạt, lớp không đổi được phiên bản khóa học.`}
          confirm="Kích hoạt"
          loading={transition.isPending}
          onConfirm={confirm}
        />
      ) : (
        <ConfirmDialog
          open={open}
          onOpenChange={setOpen}
          title={`Kết thúc lớp ${cls.code}?`}
          text="Học viên chỉ còn xem học liệu, không tích hoàn thành được nữa. Không mời thêm học viên được. Không hoàn tác được."
          confirm="Kết thúc lớp"
          danger
          loading={transition.isPending}
          onConfirm={confirm}
        />
      )}
    </>
  );
}
