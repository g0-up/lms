import { ChevronRight } from "lucide-react";
import { Fragment } from "react";
import { Link } from "react-router";
import { STATUS_VI, type VersionStatus } from "@/shared/domain";
import { cn } from "@/shared/lib/cn";
import { versionLabel } from "@/shared/lib/format";

export interface VersionRef {
  id: string;
  no: number;
  status: VersionStatus;
}

const STATUS_CLASS: Record<VersionStatus, string> = {
  published: "bg-navy-700 text-white hover:bg-navy-700",
  draft: "border-dashed",
  archived: "border-gray-300 bg-surface-100 text-gray-600 hover:bg-surface-100",
};

export interface VersionPillProps {
  version: VersionRef;
  /** Usually the same page with `?v={id}`. */
  to: string;
  current?: boolean;
  className?: string;
}

/** Rounded version link ("v2 nháp"); the version being viewed carries `aria-current="true"`. */
export function VersionPill({ version, to, current = false, className }: VersionPillProps) {
  return (
    <Link
      to={to}
      title={STATUS_VI[version.status]}
      aria-current={current ? "true" : undefined}
      className={cn(
        "inline-flex h-7 items-center gap-1.5 rounded-pill border border-navy-700 bg-white px-3 text-sm font-semibold tracking-normal text-navy-700 no-underline",
        "transition-colors duration-[180ms] ease-brand hover:bg-surface-blue-50",
        "aria-[current=true]:ring-3 aria-[current=true]:ring-surface-blue-100",
        "max-[720px]:min-h-11 max-[720px]:min-w-11 max-[720px]:justify-center",
        STATUS_CLASS[version.status],
        className,
      )}
    >
      {versionLabel(version)}
      {version.status === "draft" ? <span className="font-normal text-ink-3">nháp</span> : null}
    </Link>
  );
}

export interface LineageProps {
  versions: VersionRef[];
  hrefFor: (version: VersionRef) => string;
  currentId?: string;
  className?: string;
}

/** Version history left to right: v1 → v2 → v3. */
export function Lineage({ versions, hrefFor, currentId, className }: LineageProps) {
  return (
    <div className={cn("flex flex-wrap items-center gap-2", className)}>
      {versions.map((v, i) => (
        <Fragment key={v.id}>
          {i > 0 ? <ChevronRight aria-hidden="true" className="size-4 text-gray-300" /> : null}
          <VersionPill version={v} to={hrefFor(v)} current={v.id === currentId} />
        </Fragment>
      ))}
    </div>
  );
}
