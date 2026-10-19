import { cva, type VariantProps } from "class-variance-authority";
import type { ComponentProps } from "react";
import { STATUS_VI, SUBMISSION_STATUS_VI, type StatusKey, type SubmissionStatus } from "@/shared/domain";
import { cn } from "@/shared/lib/cn";

/** Square GoUp tag. 11px is the design system's tag size (goup `.un-tag`). */
export const badgeVariants = cva(
  // design-audit-allow font-size: goup tags (.un-tag) are 11px short labels, never body text.
  "inline-flex items-center gap-1.5 px-1.5 py-0.5 align-middle font-sans text-[11px] leading-[14px] tracking-[.5px] whitespace-nowrap",
  {
    variants: {
      variant: {
        solid: "bg-navy-700 text-white",
        outline: "border border-navy-700 bg-white text-navy-700",
        muted: "border border-gray-300 bg-surface-100 text-gray-600",
        warn: "border border-warn-line bg-warn-bg text-warn",
        subtle: "bg-surface-100 text-gray-600",
        ok: "border border-ok-line bg-ok-bg text-ok",
        info: "border border-surface-blue-100 bg-surface-blue-50 text-navy-700",
        danger: "border border-danger-line bg-danger-bg text-danger",
      },
    },
    defaultVariants: { variant: "solid" },
  },
);

export type BadgeProps = ComponentProps<"span"> & VariantProps<typeof badgeVariants>;

export function Badge({ className, variant, ...props }: BadgeProps) {
  return <span data-slot="badge" className={cn(badgeVariants({ variant }), className)} {...props} />;
}

type BadgeVariant = NonNullable<BadgeProps["variant"]>;

const STATUS_VARIANT: Partial<Record<StatusKey, BadgeVariant>> = {
  published: "solid",
  active: "solid",
  draft: "outline",
  archived: "muted",
  ended: "muted",
};

/** Version or class status badge with the prototype's Vietnamese label. */
export function StatusBadge({ status, className }: { status: StatusKey; className?: string }) {
  return (
    <Badge variant={STATUS_VARIANT[status] ?? "subtle"} className={className}>
      {STATUS_VI[status]}
    </Badge>
  );
}

const SUBMISSION_VARIANT: Record<SubmissionStatus, BadgeVariant> = {
  pending: "info",
  passed: "ok",
  rework: "warn",
  superseded: "muted",
};

/** Homework submission status badge; `children` replaces the label (e.g. "Chờ chấm · 2 giờ"). */
export function SubmissionBadge({ status, children }: { status: SubmissionStatus; children?: string }) {
  return <Badge variant={SUBMISSION_VARIANT[status]}>{children ?? SUBMISSION_STATUS_VI[status]}</Badge>;
}

/** Tone of an API status label ("Chờ chấm", "Đạt 80%"…); `optional` is a lesson not started yet. */
export type StatusTone = "ok" | "info" | "warn" | "danger" | "optional";

const TONE_VARIANT: Record<StatusTone, BadgeVariant> = {
  ok: "ok",
  info: "info",
  warn: "warn",
  danger: "danger",
  optional: "subtle",
};

/** Badge of a lesson status label as the API words it, in the variant of its tone. */
export function ToneBadge({ tone, children, className }: { tone: StatusTone; children: string; className?: string }) {
  return (
    <Badge variant={TONE_VARIANT[tone]} className={className}>
      {children}
    </Badge>
  );
}
