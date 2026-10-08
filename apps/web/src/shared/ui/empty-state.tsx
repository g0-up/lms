import type { ReactNode } from "react";
import { cn } from "@/shared/lib/cn";

export interface EmptyStateProps {
  title: string;
  text?: ReactNode;
  action?: ReactNode;
  className?: string;
}

export function EmptyState({ title, text, action, className }: EmptyStateProps) {
  return (
    <div className={cn("px-6 py-12 text-center text-ink-3", className)}>
      <h3 className="mb-1">{title}</h3>
      {text ? <p>{text}</p> : null}
      {action ? <div className="mt-4 flex justify-center">{action}</div> : null}
    </div>
  );
}
