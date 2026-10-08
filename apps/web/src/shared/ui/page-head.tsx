import type { ReactNode } from "react";
import { cn } from "@/shared/lib/cn";
import { Crumbs, type Crumb } from "./crumbs";

export interface PageHeadProps {
  title: ReactNode;
  lede?: ReactNode;
  actions?: ReactNode;
  badges?: ReactNode;
  crumbs?: Crumb[];
  className?: string;
}

/** Page header: optional crumbs, the page's only `h1`, badges, lede and actions. */
export function PageHead({ title, lede, actions, badges, crumbs, className }: PageHeadProps) {
  return (
    <>
      {crumbs?.length ? <Crumbs items={crumbs} /> : null}
      <div className={cn("mb-6 flex flex-wrap items-start justify-between gap-4", className)}>
        <div className="min-w-0">
          <div className="flex flex-wrap items-center gap-3">
            <h1 tabIndex={-1} className="focus:outline-none">
              {title}
            </h1>
            {badges}
          </div>
          {lede ? <p className="mt-1 max-w-[64ch] text-ink-3">{lede}</p> : null}
        </div>
        {actions ? (
          <div className="flex flex-wrap gap-2 max-[720px]:w-full max-[720px]:[&>*]:flex-1">{actions}</div>
        ) : null}
      </div>
    </>
  );
}
