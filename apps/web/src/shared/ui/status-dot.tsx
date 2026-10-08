import { cva, type VariantProps } from "class-variance-authority";
import { STATUS_VI } from "@/shared/domain";
import { cn } from "@/shared/lib/cn";

const dotVariants = cva("size-2 shrink-0 rounded-full", {
  variants: {
    variant: {
      ok: "bg-ok",
      sent: "bg-ok",
      active: "bg-ok",
      queued: "bg-gray-300",
      failed: "bg-danger",
      disabled: "bg-danger",
      invited: "bg-warn",
      dropped: "bg-gray-300",
    },
  },
  defaultVariants: { variant: "queued" },
});

export type StatusDotVariant = NonNullable<VariantProps<typeof dotVariants>["variant"]>;

export interface StatusDotProps {
  variant: StatusDotVariant;
  /** Defaults to the status's Vietnamese label. */
  label?: string;
  className?: string;
}

export function StatusDot({ variant, label, className }: StatusDotProps) {
  const text = label ?? (variant === "ok" ? "" : STATUS_VI[variant]);
  return (
    <span className={cn("inline-flex items-center gap-1.5 whitespace-nowrap", className)}>
      <span aria-hidden="true" className={dotVariants({ variant })} />
      {text}
    </span>
  );
}
