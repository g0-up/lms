import { cn } from "@/shared/lib/cn";
import { Checkbox } from "@/shared/ui/checkbox";

export interface TickCheckboxProps {
  lessonTitle: string;
  checked: boolean;
  /** Why ticking is not possible (see `tickBlocker`); disables the box and becomes its tooltip. */
  blocker: string | null;
  onCheckedChange: (checked: boolean) => void;
}

/** "Đã học xong" self-report box of a roadmap row. */
export function TickCheckbox({ lessonTitle, checked, blocker, onCheckedChange }: TickCheckboxProps) {
  const disabled = blocker !== null;
  return (
    <label
      title={blocker ?? undefined}
      className={cn(
        "flex min-h-11 flex-none cursor-pointer items-center gap-2 text-sm text-ink",
        disabled && "cursor-not-allowed text-ink-3",
      )}
    >
      <Checkbox
        checked={checked}
        disabled={disabled}
        aria-label={`Đã học xong: ${lessonTitle}`}
        onCheckedChange={(value) => {
          onCheckedChange(value === true);
        }}
      />
      <span>Đã học xong</span>
    </label>
  );
}
