import { cva, type VariantProps } from "class-variance-authority";
import { CircleAlert, CircleCheck, Info, TriangleAlert } from "lucide-react";
import type { ComponentProps, ReactNode } from "react";
import { cn } from "@/shared/lib/cn";

export const alertVariants = cva(
  "flex items-start gap-3 border px-4 py-3 text-sm [&>svg]:mt-px [&>svg]:size-[18px] [&>svg]:shrink-0",
  {
    variants: {
      variant: {
        info: "border-surface-blue-100 bg-surface-blue-50 text-navy-700",
        warn: "border-warn-line bg-warn-bg text-warn",
        danger: "border-danger-line bg-danger-bg text-danger",
        ok: "border-ok-line bg-ok-bg text-ok",
      },
    },
    defaultVariants: { variant: "info" },
  },
);

type AlertVariant = NonNullable<VariantProps<typeof alertVariants>["variant"]>;

const ICONS: Record<AlertVariant, ReactNode> = {
  info: <Info aria-hidden="true" />,
  warn: <TriangleAlert aria-hidden="true" />,
  danger: <CircleAlert aria-hidden="true" />,
  ok: <CircleCheck aria-hidden="true" />,
};

export type AlertProps = ComponentProps<"div"> &
  VariantProps<typeof alertVariants> & {
    /** Leading icon; defaults to the variant's icon, `false` hides it. */
    icon?: ReactNode | false;
  };

/** Inline message. Pass `role="alert"` for errors raised by a user action. */
export function Alert({ className, variant, icon, children, ...props }: AlertProps) {
  const leading = icon === false ? null : (icon ?? ICONS[variant ?? "info"]);
  return (
    <div data-slot="alert" className={cn(alertVariants({ variant }), className)} {...props}>
      {leading}
      <div className="min-w-0 flex-1">{children}</div>
    </div>
  );
}

export function AlertTitle({ className, ...props }: ComponentProps<"strong">) {
  return <strong data-slot="alert-title" className={cn("block font-semibold", className)} {...props} />;
}

export function AlertDescription({ className, ...props }: ComponentProps<"div">) {
  return <div data-slot="alert-description" className={cn(className)} {...props} />;
}

export function AlertActions({ className, ...props }: ComponentProps<"div">) {
  return <div data-slot="alert-actions" className={cn("mt-2 flex flex-wrap gap-2", className)} {...props} />;
}
