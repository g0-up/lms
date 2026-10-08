import { useId, useState, type SubmitEvent } from "react";
import { Link } from "react-router";
import { Button } from "@/shared/ui/button";
import { Checkbox } from "@/shared/ui/checkbox";
import { Field } from "@/shared/ui/field";
import { Input } from "@/shared/ui/input";
import { Label } from "@/shared/ui/label";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/shared/ui/select";
import { filterFromForm, hasAnyFilter, type ReportFilter } from "../model/report-filter";
import { SORT_OPTIONS } from "../model/sort";

export interface FilterBarProps {
  /** The filter in the URL; remount the bar (`key`) when it changes so the fields follow it. */
  filter: ReportFilter;
  /** Page without any filter, the target of "Xóa bộ lọc". */
  clearTo: string;
  onApply: (filter: ReportFilter) => void;
}

// Compact 36px controls on desktop as in the prototype's `.filter-bar`; touch screens keep 44px and 16px text.
const compact = "h-9 min-[721px]:text-sm";
const checkLabel = "flex min-h-9 cursor-pointer items-center gap-2 text-sm text-ink max-[720px]:min-h-11";

/** Report filter form; nothing is applied until "Lọc", as in a GET form. */
export function FilterBar({ filter, clearTo, onApply }: FilterBarProps) {
  const sortId = useId();
  const [notLoggedIn, setNotLoggedIn] = useState(filter.notLoggedIn);
  const [inactive, setInactive] = useState(filter.inactiveDays === null ? "" : String(filter.inactiveDays));
  const [below, setBelow] = useState(filter.belowPercent === null ? "" : String(filter.belowPercent));
  const [includeDropped, setIncludeDropped] = useState(filter.includeDropped);
  const [sort, setSort] = useState<string>(filter.sort);

  const submit = (event: SubmitEvent<HTMLFormElement>) => {
    event.preventDefault();
    onApply(filterFromForm({ notLoggedIn, inactive, below, includeDropped, sort }));
  };

  return (
    <form
      role="search"
      aria-label="Lọc báo cáo"
      onSubmit={submit}
      className="flex flex-wrap items-end gap-4 border-b border-line px-5 py-4 max-[720px]:px-4"
    >
      <label className={checkLabel}>
        <Checkbox checked={notLoggedIn} onCheckedChange={(v) => { setNotLoggedIn(v === true); }} />
        Chưa đăng nhập
      </label>
      <Field label="Không hoạt động quá (ngày)" className="min-w-[160px]">
        <Input
          type="number"
          inputMode="numeric"
          min={1}
          max={3650}
          placeholder="7"
          className={compact}
          value={inactive}
          onChange={(e) => { setInactive(e.target.value); }}
        />
      </Field>
      <Field label="% toàn khóa dưới" className="min-w-[160px]">
        <Input
          type="number"
          inputMode="numeric"
          min={1}
          max={100}
          placeholder="50"
          className={compact}
          value={below}
          onChange={(e) => { setBelow(e.target.value); }}
        />
      </Field>
      <label className={checkLabel}>
        <Checkbox checked={includeDropped} onCheckedChange={(v) => { setIncludeDropped(v === true); }} />
        Hiện học viên đã rời lớp
      </label>
      <div className="grid min-w-[200px] gap-1.5">
        <Label htmlFor={sortId}>Sắp xếp</Label>
        <Select value={sort} onValueChange={setSort}>
          <SelectTrigger id={sortId} className={compact}>
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            {SORT_OPTIONS.map((o) => (
              <SelectItem key={o.value} value={o.value}>
                {o.label}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      </div>
      <div className="flex flex-wrap items-center gap-2">
        <Button type="submit" size="sm">
          Lọc
        </Button>
        {hasAnyFilter(filter) ? (
          <Button asChild variant="ghost" size="sm">
            <Link to={clearTo}>Xóa bộ lọc</Link>
          </Button>
        ) : null}
      </div>
    </form>
  );
}
