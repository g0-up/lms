import { cva, type VariantProps } from "class-variance-authority";
import { Slot } from "radix-ui";
import type { ComponentProps } from "react";
import { cn } from "@/shared/lib/cn";

/** GoUp pill button. One `gradient` button per page: the page's single primary action. */
export const buttonVariants = cva(
  [
    "inline-flex shrink-0 cursor-pointer items-center justify-center gap-2 whitespace-nowrap rounded-full font-sans",
    "text-[13px] leading-[13px] font-medium tracking-[1.3px] uppercase no-underline",
    "min-h-11 px-[35px] py-[18px] transition-[filter,background-color,color] duration-200",
    "enabled:active:translate-y-px disabled:cursor-not-allowed disabled:opacity-45 disabled:filter-none",
    "aria-disabled:cursor-not-allowed aria-disabled:opacity-45",
    "focus-visible:outline-2 focus-visible:outline-offset-3 focus-visible:outline-navy-700",
    "[&_svg]:pointer-events-none [&_svg]:size-4 [&_svg]:shrink-0",
    "max-[400px]:max-w-full max-[400px]:text-center max-[400px]:leading-[1.3] max-[400px]:whitespace-normal",
  ],
  {
    variants: {
      variant: {
        default: "bg-navy-700 text-white hover:bg-navy-900",
        gradient: "bg-navy-700 bg-(image:--gradient-accent) text-on-accent hover:brightness-106",
        outline: "border border-navy-700 bg-white text-navy-700 hover:bg-surface-blue-50",
        ghost: "bg-transparent px-3 text-navy-700 hover:bg-surface-blue-50",
        destructive: "bg-destructive text-white hover:brightness-92",
        link: [
          "rounded-none bg-transparent px-0 py-0 text-sm leading-normal font-normal tracking-normal normal-case",
          "text-navy-700 underline hover:text-navy-900",
        ],
      },
      size: {
        default: "",
        sm: "min-h-9 px-4 py-2.5 text-xs tracking-[1px] max-[720px]:min-h-11",
        icon: "size-11 rounded-sm p-0",
      },
    },
    defaultVariants: { variant: "default", size: "default" },
  },
);

export type ButtonProps = ComponentProps<"button"> &
  VariantProps<typeof buttonVariants> & {
    asChild?: boolean;
  };

export function Button({ className, variant, size, asChild = false, type, ...props }: ButtonProps) {
  const Comp = asChild ? Slot.Root : "button";
  return (
    <Comp
      data-slot="button"
      type={asChild ? undefined : (type ?? "button")}
      className={cn(buttonVariants({ variant, size }), className)}
      {...props}
    />
  );
}
