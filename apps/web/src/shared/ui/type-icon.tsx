import { FileText, Play } from "lucide-react";
import { cn } from "@/shared/lib/cn";

export type LessonType = "video" | "markdown";

/** 32px square marking a lesson as Video or Markdown. */
export function TypeIcon({ type, className }: { type: LessonType; className?: string }) {
  const label = type === "video" ? "Video" : "Markdown";
  const Icon = type === "video" ? Play : FileText;
  return (
    <span
      title={label}
      role="img"
      aria-label={label}
      className={cn("inline-grid size-8 shrink-0 place-items-center bg-surface-blue-50 text-navy-700", className)}
    >
      <Icon className="size-4" aria-hidden="true" />
    </span>
  );
}
