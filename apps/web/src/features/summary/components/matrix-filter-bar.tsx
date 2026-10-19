import { useId, useState, type SubmitEvent } from "react";
import { Link } from "react-router";
import { Button } from "@/shared/ui/button";
import { Checkbox } from "@/shared/ui/checkbox";
import { Label } from "@/shared/ui/label";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/shared/ui/select";
import { ALL_STAGES, hasMatrixFilter, matrixFilterFromForm, type MatrixFilter } from "../model/matrix-filter";
import type { StageRef } from "../model/schemas";

export interface MatrixFilterBarProps {
  /** The filter in the URL; remount the bar (`key`) when it changes so the fields follow it. */
  filter: MatrixFilter;
  stages: StageRef[];
  /** Tab without any filter, the target of "Xóa bộ lọc". */
  clearTo: string;
  onApply: (filter: MatrixFilter) => void;
}

// Compact 36px controls on desktop as in the prototype's `.filter-bar`; touch screens keep 44px and 16px text.
const compact = "h-9 min-[721px]:text-sm";
const checkLabel = "flex min-h-9 cursor-pointer items-center gap-2 text-sm text-ink max-[720px]:min-h-11";

function StageSelect({
  label,
  value,
  stages,
  onChange,
}: {
  label: string;
  value: string;
  stages: StageRef[];
  onChange: (value: string) => void;
}) {
  const id = useId();
  return (
    <div className="grid min-w-[200px] gap-1.5">
      <Label htmlFor={id}>{label}</Label>
      <Select value={value} onValueChange={onChange}>
        <SelectTrigger id={id} className={compact}>
          <SelectValue />
        </SelectTrigger>
        <SelectContent>
          <SelectItem value={ALL_STAGES}>Tất cả chặng</SelectItem>
          {stages.map((s) => (
            <SelectItem key={s.stageVersionId} value={s.stageVersionId}>
              {s.name}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
    </div>
  );
}

/** Summary matrix filter; nothing is applied until "Lọc", as in a GET form. */
export function MatrixFilterBar({ filter, stages, clearTo, onApply }: MatrixFilterBarProps) {
  const [stage, setStage] = useState(filter.stage ?? ALL_STAGES);
  const [noReview, setNoReview] = useState(filter.noReview ?? ALL_STAGES);
  const [rework, setRework] = useState(filter.rework);
  const [overdue, setOverdue] = useState(filter.overdue);

  const submit = (event: SubmitEvent<HTMLFormElement>) => {
    event.preventDefault();
    onApply(matrixFilterFromForm({ stage, noReview, rework, overdue }));
  };

  return (
    <form
      role="search"
      aria-label="Lọc tổng kết"
      onSubmit={submit}
      className="flex flex-wrap items-end gap-4 border-b border-line px-5 py-4 max-[720px]:px-4"
    >
      <StageSelect label="Chưa hoàn thành chặng" value={stage} stages={stages} onChange={setStage} />
      <StageSelect label="Chưa có nhận xét chặng" value={noReview} stages={stages} onChange={setNoReview} />
      <label className={checkLabel}>
        <Checkbox checked={rework} onCheckedChange={(v) => { setRework(v === true); }} />
        Có bài cần làm lại
      </label>
      <label className={checkLabel}>
        <Checkbox checked={overdue} onCheckedChange={(v) => { setOverdue(v === true); }} />
        Có bài quá hạn nộp
      </label>
      <div className="flex flex-wrap items-center gap-2">
        <Button type="submit" size="sm">
          Lọc
        </Button>
        {hasMatrixFilter(filter) ? (
          <Button asChild variant="ghost" size="sm">
            <Link to={clearTo}>Xóa bộ lọc</Link>
          </Button>
        ) : null}
      </div>
    </form>
  );
}
