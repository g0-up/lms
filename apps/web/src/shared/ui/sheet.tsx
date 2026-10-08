import { X } from "lucide-react";
import { Dialog as SheetPrimitive } from "radix-ui";
import type { ComponentProps } from "react";
import { cn } from "@/shared/lib/cn";
import { IconButton } from "./icon-button";

export const Sheet = SheetPrimitive.Root;
export const SheetTrigger = SheetPrimitive.Trigger;
export const SheetClose = SheetPrimitive.Close;

const SIDE = {
  right: "inset-y-0 right-0 w-[min(520px,100vw)] max-[960px]:w-full data-[state=open]:animate-slide",
  left: "inset-y-0 left-0 w-full data-[state=open]:animate-in data-[state=open]:slide-in-from-left-6 data-[state=open]:fade-in-0",
} as const;

export type SheetContentProps = ComponentProps<typeof SheetPrimitive.Content> & {
  side?: keyof typeof SIDE;
  showClose?: boolean;
};

/** Side panel on the float shadow; right drawer is 520px and full width at ≤960px. */
export function SheetContent({ className, children, side = "right", showClose = true, ...props }: SheetContentProps) {
  return (
    <SheetPrimitive.Portal>
      <SheetPrimitive.Overlay
        data-slot="sheet-overlay"
        className="fixed inset-0 z-50 bg-navy-900/45 data-[state=open]:animate-in data-[state=open]:fade-in-0 data-[state=closed]:animate-out data-[state=closed]:fade-out-0"
      />
      <SheetPrimitive.Content
        data-slot="sheet-content"
        className={cn(
          "fixed z-50 flex flex-col bg-white shadow-float focus:outline-none",
          "data-[state=closed]:animate-out data-[state=closed]:fade-out-0",
          SIDE[side],
          className,
        )}
        {...props}
      >
        {children}
        {showClose ? (
          <SheetPrimitive.Close asChild>
            <IconButton aria-label="Đóng" className="absolute top-2 right-2">
              <X aria-hidden="true" />
            </IconButton>
          </SheetPrimitive.Close>
        ) : null}
      </SheetPrimitive.Content>
    </SheetPrimitive.Portal>
  );
}

export function SheetHeader({ className, ...props }: ComponentProps<"div">) {
  return (
    <div
      data-slot="sheet-header"
      className={cn("flex min-h-15 items-center gap-3 border-b border-line py-4 pr-14 pl-6", className)}
      {...props}
    />
  );
}

/** Scrolling body; focusable so the keyboard can scroll it when it holds no control. Name it with `aria-label`. */
export function SheetBody({ className, ...props }: ComponentProps<"div">) {
  return (
    <div
      data-slot="sheet-body"
      tabIndex={0}
      className={cn(
        "flex-1 overflow-auto focus-visible:outline-2 focus-visible:-outline-offset-2 focus-visible:outline-navy-700",
        className,
      )}
      {...props}
    />
  );
}

export function SheetFooter({ className, ...props }: ComponentProps<"div">) {
  return (
    <div
      data-slot="sheet-footer"
      className={cn("flex flex-wrap justify-end gap-2 border-t border-line px-6 py-4", className)}
      {...props}
    />
  );
}

export function SheetTitle({ className, ...props }: ComponentProps<typeof SheetPrimitive.Title>) {
  return <SheetPrimitive.Title data-slot="sheet-title" className={cn("text-lg font-semibold text-ink", className)} {...props} />;
}

export function SheetDescription({ className, ...props }: ComponentProps<typeof SheetPrimitive.Description>) {
  return <SheetPrimitive.Description data-slot="sheet-description" className={cn("text-ink-3", className)} {...props} />;
}
