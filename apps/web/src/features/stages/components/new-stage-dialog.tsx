import type { CodeNameOutput } from "../model/code-name-form";
import { CodeNameDialog } from "./code-name-dialog";

export interface NewStageDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onCreate: (values: CodeNameOutput) => Promise<unknown>;
}

export function NewStageDialog(props: NewStageDialogProps) {
  return (
    <CodeNameDialog
      {...props}
      title="Tạo chặng"
      submitLabel="Tạo chặng"
      codeLabel="Mã chặng"
      codePlaceholder="vd: DOCKER"
      codeHelp="Duy nhất, dùng trong báo cáo."
      nameLabel="Tên chặng"
      namePlaceholder="vd: Docker cơ bản"
      note="Hệ thống tạo kèm phiên bản nháp v1 để bạn thêm học liệu."
      requiredMessage="Nhập mã và tên chặng."
      conflictMessage="Mã chặng đã tồn tại."
    />
  );
}
