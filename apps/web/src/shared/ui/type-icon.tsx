import { CircleHelp, ClipboardList, FileText, Play, type LucideIcon } from "lucide-react";
import { cn } from "@/shared/lib/cn";

export type LessonType = "video" | "markdown" | "quiz" | "homework";

const TYPES: Record<LessonType, { label: string; Icon: LucideIcon }> = {
  video: { label: "Video", Icon: Play },
  markdown: { label: "Markdown", Icon: FileText },
  quiz: { label: "Quiz", Icon: CircleHelp },
  homework: { label: "Bài tập", Icon: ClipboardList },
};

/** 32px square marking a lesson's type. */
export function TypeIcon({ type, className }: { type: LessonType; className?: string }) {
  const { label, Icon } = TYPES[type];
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
