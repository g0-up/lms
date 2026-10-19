import { ConfirmDialog } from "@/shared/ui/confirm-dialog";

export interface QuizSubmitDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  unanswered: number;
  loading: boolean;
  onConfirm: () => void;
}

/** "Nộp bài?" confirmation, naming how many questions are still blank. */
export function QuizSubmitDialog({ open, onOpenChange, unanswered, loading, onConfirm }: QuizSubmitDialogProps) {
  return (
    <ConfirmDialog
      open={open}
      onOpenChange={onOpenChange}
      title="Nộp bài?"
      text={
        unanswered > 0 ? (
          <>
            Bạn còn <strong>{unanswered} câu chưa trả lời</strong>; các câu này tính 0 điểm. Sau khi nộp không sửa
            được nữa.
          </>
        ) : (
          "Bạn đã trả lời tất cả câu hỏi. Sau khi nộp không sửa được nữa."
        )
      }
      confirm="Nộp bài"
      loading={loading}
      onConfirm={onConfirm}
    />
  );
}
