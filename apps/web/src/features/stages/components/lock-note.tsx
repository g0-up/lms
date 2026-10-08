import { Lock } from "lucide-react";

/** Marks a non-draft version's list as read-only. */
export function LockNote() {
  return (
    <span className="inline-flex items-center gap-1.5 text-sm text-ink-3">
      <Lock aria-hidden="true" className="size-4" /> Không sửa được
    </span>
  );
}
