import { useId, useState, type SubmitEvent } from "react";
import { Link } from "react-router";
import { GRADING_SLA_DAYS, shortTitle } from "@/shared/domain";
import { Button } from "@/shared/ui/button";
import { Checkbox } from "@/shared/ui/checkbox";
import { Label } from "@/shared/ui/label";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/shared/ui/select";
import { ALL, hasAnyQueueFilter, queueFilterFromForm, type QueueFilter } from "../model/queue-filter";
import type { Queue } from "../model/schemas";

export interface QueueFilterBarProps {
  /** The filter in the URL; remount the bar (`key`) when it changes so the fields follow it. */
  filter: QueueFilter;
  /** Classes and lessons the grader may filter on, as the API lists them. */
  options: Queue["filters"];
  /** Queue without any filter, the target of "Xóa bộ lọc". */
  clearTo: string;
  onApply: (filter: QueueFilter) => void;
}

const KIND_OPTIONS = [
  { value: ALL, label: "Tất cả" },
  { value: "homework", label: "Bài tập" },
  { value: "quiz", label: "Quiz bài tập" },
];

// Compact 36px controls on desktop as in the prototype's `.filter-bar`; touch screens keep 44px and 16px text.
const compact = "h-9 min-[721px]:text-sm";
const checkLabel = "flex min-h-9 cursor-pointer items-center gap-2 text-sm text-ink max-[720px]:min-h-11";

interface FilterSelectProps {
  label: string;
  value: string;
  onChange: (value: string) => void;
  options: { value: string; label: string }[];
}

function FilterSelect({ label, value, onChange, options }: FilterSelectProps) {
  const id = useId();
  return (
    <div className="grid min-w-[160px] gap-1.5">
      <Label htmlFor={id}>{label}</Label>
      <Select value={value} onValueChange={onChange}>
        <SelectTrigger id={id} className={compact}>
          <SelectValue />
        </SelectTrigger>
        <SelectContent>
          {options.map((o) => (
            <SelectItem key={o.value} value={o.value}>
              {o.label}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
    </div>
  );
}

/** Grading queue filter; nothing is applied until "Lọc", as in a GET form. */
export function QueueFilterBar({ filter, options, clearTo, onApply }: QueueFilterBarProps) {
  const [type, setType] = useState(filter.kind ?? ALL);
  const [classId, setClassId] = useState(filter.classId ?? ALL);
  const [lessonId, setLessonId] = useState(filter.lessonId ?? ALL);
  const [overdue, setOverdue] = useState(filter.overdue);

  const submit = (event: SubmitEvent<HTMLFormElement>) => {
    event.preventDefault();
    onApply(queueFilterFromForm({ type, classId, lessonId, overdue }));
  };

  return (
    <form
      role="search"
      aria-label="Lọc hàng chờ"
      onSubmit={submit}
      className="flex flex-wrap items-end gap-4 border-b border-line px-5 py-4 max-[720px]:px-4"
    >
      <FilterSelect label="Loại" value={type} onChange={setType} options={KIND_OPTIONS} />
      <FilterSelect
        label="Lớp"
        value={classId}
        onChange={setClassId}
        options={[{ value: ALL, label: "Tất cả lớp" }, ...options.classes.map((c) => ({ value: c.id, label: c.code }))]}
      />
      <FilterSelect
        label="Bài"
        value={lessonId}
        onChange={setLessonId}
        options={[
          { value: ALL, label: "Tất cả bài" },
          ...options.lessons.map((l) => ({
            value: l.id,
            label: `${shortTitle(l.title)} (${l.kind === "quiz" ? "quiz" : "bài tập"})`,
          })),
        ]}
      />
      <label className={checkLabel}>
        <Checkbox checked={overdue} onCheckedChange={(v) => { setOverdue(v === true); }} />
        Chỉ mục quá {GRADING_SLA_DAYS} ngày
      </label>
      <div className="flex flex-wrap items-center gap-2">
        <Button type="submit" size="sm">
          Lọc
        </Button>
        {hasAnyQueueFilter(filter) ? (
          <Button asChild variant="ghost" size="sm">
            <Link to={clearTo}>Xóa bộ lọc</Link>
          </Button>
        ) : null}
      </div>
    </form>
  );
}
