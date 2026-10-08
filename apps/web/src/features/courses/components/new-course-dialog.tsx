import { CodeNameDialog, type CodeNameOutput } from "@/features/stages";

export interface NewCourseDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onCreate: (values: CodeNameOutput) => Promise<unknown>;
}

export function NewCourseDialog(props: NewCourseDialogProps) {
  return (
    <CodeNameDialog
      {...props}
      title="Tạo khóa học"
      submitLabel="Tạo khóa học"
      codeLabel="Mã khóa học"
      codePlaceholder="vd: ADV"
      nameLabel="Tên khóa học"
      namePlaceholder="vd: Lập trình nâng cao"
      note="Hệ thống tạo kèm phiên bản nháp v1 để bạn ghép chặng."
      requiredMessage="Nhập mã và tên khóa học."
      conflictMessage="Mã khóa học đã tồn tại."
    />
  );
}
