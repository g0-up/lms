import type { ReactNode } from "react";
import { cn } from "@/shared/lib/cn";
import { Card } from "./card";

export interface KpiCardProps {
  label: string;
  value: ReactNode;
  hint?: ReactNode;
  className?: string;
}

export function KpiCard({ label, value, hint, className }: KpiCardProps) {
  return (
    <Card className={cn("px-6 py-4", className)}>
      <div className="text-sm tracking-[.5px] text-ink-3 uppercase">{label}</div>
      <div className="mt-1 text-2xl leading-[1.1] font-semibold tracking-[-.5px] text-navy-900 tabular-nums">{value}</div>
      {hint ? <div className="mt-1 text-sm text-ink-3">{hint}</div> : null}
    </Card>
  );
}
