import { Pencil, Plus, Trash2 } from "lucide-react";
import { useState } from "react";
import { Button } from "@/shared/ui/button";
import { Card, CardHeader, CardTitle } from "@/shared/ui/card";
import { ConfirmDialog } from "@/shared/ui/confirm-dialog";
import { EmptyState } from "@/shared/ui/empty-state";
import { IconButton } from "@/shared/ui/icon-button";
import { Table, TableBody, TableCaption, TableCell, TableHead, TableHeader, TableRow } from "@/shared/ui/table";
import { TableWrap } from "@/shared/ui/table-wrap";
import { useCriterionMutations } from "../hooks/use-criterion-mutations";
import { criteriaCountLabel } from "../model/criterion-form";
import { moveItem } from "../model/order";
import type { Criterion, Homework } from "../model/schemas";
import { CriterionFormDialog } from "./criterion-form-dialog";
import { OrderButtons } from "./order-buttons";

export interface CriteriaTableProps {
  stageId: string;
  vid: string;
  lessonId: string;
  homework: Homework;
  /** Only a draft shows the add, reorder, edit and delete controls. */
  editable: boolean;
}

type Dialog = { kind: "form"; criterion?: Criterion } | { kind: "delete"; criterion: Criterion } | null;

/** "Tiêu chí chấm" card of a homework: the rubric with its total, which is the homework's maximum score. */
export function CriteriaTable({ stageId, vid, lessonId, homework, editable }: CriteriaTableProps) {
  const { save, remove, reorder } = useCriterionMutations(stageId, vid, lessonId);
  const [dialog, setDialog] = useState<Dialog>(null);
  const { criteria } = homework;
  const close = () => {
    setDialog(null);
  };
  const add = () => {
    setDialog({ kind: "form" });
  };
  const editing = dialog?.kind === "form" ? dialog.criterion : undefined;

  return (
    <Card>
      <CardHeader>
        <CardTitle>
          Tiêu chí chấm <span className="text-sm font-normal text-ink-3">{criteriaCountLabel(homework)}</span>
        </CardTitle>
        {editable && criteria.length > 0 ? (
          <Button variant="outline" size="sm" onClick={add}>
            <Plus aria-hidden="true" /> Thêm tiêu chí
          </Button>
        ) : null}
      </CardHeader>
      {criteria.length === 0 ? (
        <EmptyState
          title="Chưa có tiêu chí"
          text="Bài tập cần ít nhất một tiêu chí chấm trước khi phát hành chặng."
          action={
            editable ? (
              <Button size="sm" onClick={add}>
                <Plus aria-hidden="true" /> Thêm tiêu chí đầu tiên
              </Button>
            ) : undefined
          }
        />
      ) : (
        <TableWrap>
          <Table>
            <TableCaption>Tiêu chí chấm</TableCaption>
            <TableHeader>
              <TableRow>
                <TableHead className="w-12 text-right">#</TableHead>
                <TableHead>Tiêu chí</TableHead>
                <TableHead className="text-right">Điểm tối đa</TableHead>
                {editable ? <TableHead className="text-right">Thao tác</TableHead> : null}
              </TableRow>
            </TableHeader>
            <TableBody>
              {criteria.map((criterion, i) => {
                const n = i + 1;
                return (
                  <TableRow key={criterion.id}>
                    <TableCell className="text-right tabular-nums">{n}</TableCell>
                    <TableCell>{criterion.description}</TableCell>
                    <TableCell className="text-right tabular-nums">{criterion.maxPoints}</TableCell>
                    {editable ? (
                      <TableCell className="py-1">
                        <div className="flex justify-end gap-0.5">
                          <OrderButtons
                            index={i}
                            count={criteria.length}
                            labels={{
                              up: `Chuyển tiêu chí ${String(n)} lên`,
                              down: `Chuyển tiêu chí ${String(n)} xuống`,
                            }}
                            disabled={reorder.isPending}
                            onMove={(delta) => {
                              reorder.mutate(moveItem(criteria, i, delta).map((c) => c.id));
                            }}
                          />
                          <IconButton
                            aria-label={`Sửa tiêu chí ${String(n)}`}
                            onClick={() => {
                              setDialog({ kind: "form", criterion });
                            }}
                          >
                            <Pencil aria-hidden="true" />
                          </IconButton>
                          <IconButton
                            aria-label={`Xóa tiêu chí ${String(n)}`}
                            onClick={() => {
                              setDialog({ kind: "delete", criterion });
                            }}
                          >
                            <Trash2 aria-hidden="true" />
                          </IconButton>
                        </div>
                      </TableCell>
                    ) : null}
                  </TableRow>
                );
              })}
            </TableBody>
            <tfoot>
              <TableRow>
                <TableCell className="border-b-0" />
                <TableHead scope="row" className="static border-b-0">
                  Tổng
                </TableHead>
                <TableCell className="border-b-0 text-right font-semibold tabular-nums">{homework.maxPoints}</TableCell>
                {editable ? <TableCell className="border-b-0" /> : null}
              </TableRow>
            </tfoot>
          </Table>
        </TableWrap>
      )}
      <CriterionFormDialog
        open={dialog?.kind === "form"}
        onOpenChange={close}
        criterion={editing}
        onSave={(body) => save.mutateAsync({ criterionId: editing?.id, body })}
      />
      <ConfirmDialog
        open={dialog?.kind === "delete"}
        onOpenChange={close}
        danger
        title="Xóa tiêu chí?"
        text="Tiêu chí bị xóa khỏi bản nháp này; tổng điểm bài tập giảm theo."
        confirm="Xóa tiêu chí"
        loading={remove.isPending}
        onConfirm={() => {
          if (dialog?.kind === "delete") remove.mutate(dialog.criterion.id, { onSettled: close });
        }}
      />
    </Card>
  );
}
