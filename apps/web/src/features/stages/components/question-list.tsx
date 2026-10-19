import { CircleCheck, Info, Pencil, Plus, Trash2 } from "lucide-react";
import { useState } from "react";
import { cn } from "@/shared/lib/cn";
import { Badge } from "@/shared/ui/badge";
import { Button } from "@/shared/ui/button";
import { Card, CardHeader, CardTitle } from "@/shared/ui/card";
import { ConfirmDialog } from "@/shared/ui/confirm-dialog";
import { EmptyState } from "@/shared/ui/empty-state";
import { IconButton } from "@/shared/ui/icon-button";
import { MarkdownContent } from "@/shared/ui/markdown-content";
import { useQuestionMutations } from "../hooks/use-question-mutations";
import { moveItem } from "../model/order";
import { QUESTION_TYPE_LABEL, lacksCorrectAnswer, questionCountLabel } from "../model/question-form";
import type { Question, Quiz } from "../model/schemas";
import { ItemBody, ItemMeta, List, ListItem, Ord } from "./list";
import { OrderButtons } from "./order-buttons";
import { QuestionFormDialog } from "./question-form-dialog";

export interface QuestionListProps {
  stageId: string;
  vid: string;
  lessonId: string;
  quiz: Quiz;
  /** Only a draft shows the add, reorder, edit and delete controls. */
  editable: boolean;
}

type Dialog = { kind: "form"; question?: Question } | { kind: "delete"; question: Question } | null;

/** Markdown once published (rendered by the server), the source as typed in a draft. */
function Prose({ source, html, className }: { source: string; html?: string; className?: string }) {
  if (html !== undefined) return <MarkdownContent html={html} className={className} />;
  return <div className={cn("whitespace-pre-wrap", className)}>{source}</div>;
}

/** "Câu hỏi" card of a quiz: each question with its options and the correct ones marked. */
export function QuestionList({ stageId, vid, lessonId, quiz, editable }: QuestionListProps) {
  const { save, remove, reorder } = useQuestionMutations(stageId, vid, lessonId);
  const [dialog, setDialog] = useState<Dialog>(null);
  const { questions } = quiz;
  const close = () => {
    setDialog(null);
  };
  const add = () => {
    setDialog({ kind: "form" });
  };
  const editing = dialog?.kind === "form" ? dialog.question : undefined;

  return (
    <Card>
      <CardHeader>
        <CardTitle>
          Câu hỏi <span className="text-sm font-normal text-ink-3">{questionCountLabel(questions)}</span>
        </CardTitle>
        {editable && questions.length > 0 ? (
          <Button variant="outline" size="sm" onClick={add}>
            <Plus aria-hidden="true" /> Thêm câu hỏi
          </Button>
        ) : null}
      </CardHeader>
      {questions.length === 0 ? (
        <EmptyState
          title="Chưa có câu hỏi"
          text="Quiz cần ít nhất một câu hỏi có đáp án đúng trước khi phát hành chặng."
          action={
            editable ? (
              <Button size="sm" onClick={add}>
                <Plus aria-hidden="true" /> Thêm câu hỏi đầu tiên
              </Button>
            ) : undefined
          }
        />
      ) : (
        <List>
          {questions.map((question, i) => {
            const n = i + 1;
            return (
              <ListItem key={question.id} className="items-start max-[720px]:flex-wrap">
                <Ord n={n} />
                <ItemBody className="grid gap-2">
                  <Prose source={question.bodySource} html={question.bodyHtml} className="font-medium text-ink" />
                  <ItemMeta>
                    <span>{QUESTION_TYPE_LABEL[question.type]}</span>
                    <span>{question.points} điểm</span>
                    {lacksCorrectAnswer(question) ? <Badge variant="warn">Chưa có đáp án đúng</Badge> : null}
                    <span>key {question.questionKey}</span>
                  </ItemMeta>
                  <ul className="m-0 grid list-none gap-1 p-0 text-sm">
                    {question.options.map((option) => (
                      <li key={option.id} className="flex items-start gap-2">
                        {option.isCorrect ? (
                          <>
                            <CircleCheck className="mt-0.5 size-4 shrink-0 text-ok" aria-hidden="true" />
                            <span className="sr-only">Đáp án đúng: </span>
                          </>
                        ) : (
                          <span
                            aria-hidden="true"
                            className="mt-1.5 size-2 shrink-0 rounded-full border border-gray-300"
                          />
                        )}
                        <span className={option.isCorrect ? "font-medium text-ink" : "text-ink-2"}>{option.body}</span>
                      </li>
                    ))}
                  </ul>
                  {question.explanationSource.trim() !== "" ? (
                    <div className="flex items-start gap-2 text-xs text-ink-3">
                      <Info className="mt-0.5 size-3.5 shrink-0" aria-hidden="true" />
                      <span className="sr-only">Giải thích: </span>
                      <Prose source={question.explanationSource} html={question.explanationHtml} />
                    </div>
                  ) : null}
                </ItemBody>
                {editable ? (
                  <div className="flex shrink-0 gap-0.5 max-[720px]:flex-[1_1_100%] max-[720px]:justify-end">
                    <OrderButtons
                      index={i}
                      count={questions.length}
                      labels={{ up: `Chuyển câu ${String(n)} lên`, down: `Chuyển câu ${String(n)} xuống` }}
                      disabled={reorder.isPending}
                      onMove={(delta) => {
                        reorder.mutate(moveItem(questions, i, delta).map((q) => q.id));
                      }}
                    />
                    <IconButton
                      aria-label={`Sửa câu ${String(n)}`}
                      onClick={() => {
                        setDialog({ kind: "form", question });
                      }}
                    >
                      <Pencil aria-hidden="true" />
                    </IconButton>
                    <IconButton
                      aria-label={`Xóa câu ${String(n)}`}
                      onClick={() => {
                        setDialog({ kind: "delete", question });
                      }}
                    >
                      <Trash2 aria-hidden="true" />
                    </IconButton>
                  </div>
                ) : null}
              </ListItem>
            );
          })}
        </List>
      )}
      <QuestionFormDialog
        open={dialog?.kind === "form"}
        onOpenChange={close}
        question={editing}
        onSave={(body) => save.mutateAsync({ questionId: editing?.id, body })}
      />
      <ConfirmDialog
        open={dialog?.kind === "delete"}
        onOpenChange={close}
        danger
        title="Xóa câu hỏi?"
        text="Câu hỏi bị xóa khỏi bản nháp này. Phiên bản đã phát hành không bị ảnh hưởng."
        confirm="Xóa câu hỏi"
        loading={remove.isPending}
        onConfirm={() => {
          if (dialog?.kind === "delete") remove.mutate(dialog.question.id, { onSettled: close });
        }}
      />
    </Card>
  );
}
