import { X } from "lucide-react";
import { Dialog as DialogPrimitive } from "radix-ui";
import { useRef, type ComponentProps } from "react";
import { cn } from "@/shared/lib/cn";
import { IconButton } from "./icon-button";

export const Dialog = DialogPrimitive.Root;
export const DialogTrigger = DialogPrimitive.Trigger;
export const DialogPortal = DialogPrimitive.Portal;
export const DialogClose = DialogPrimitive.Close;

export function DialogOverlay({ className, ...props }: ComponentProps<typeof DialogPrimitive.Overlay>) {
  return (
    <DialogPrimitive.Overlay
      data-slot="dialog-overlay"
      className={cn(
        "fixed inset-0 z-50 bg-navy-900/45",
        "data-[state=open]:animate-in data-[state=open]:fade-in-0 data-[state=closed]:animate-out data-[state=closed]:fade-out-0",
        className,
      )}
      {...props}
    />
  );
}

/**
 * Centered modal, 560px wide, square with the float shadow.
 *
 * Dialogs open from state rather than a Radix `DialogTrigger`, so Radix has nothing to refocus on
 * close: the element that had focus when the dialog opened gets it back, if it is still on the page.
 */
export function DialogContent({
  className,
  children,
  onOpenAutoFocus,
  onCloseAutoFocus,
  ...props
}: ComponentProps<typeof DialogPrimitive.Content>) {
  const opener = useRef<HTMLElement | null>(null);
  return (
    <DialogPortal>
      <DialogOverlay />
      <DialogPrimitive.Content
        data-slot="dialog-content"
        onOpenAutoFocus={(event) => {
          opener.current = document.activeElement instanceof HTMLElement ? document.activeElement : null;
          onOpenAutoFocus?.(event);
        }}
        onCloseAutoFocus={(event) => {
          onCloseAutoFocus?.(event);
          if (event.defaultPrevented || !opener.current?.isConnected) return;
          event.preventDefault();
          opener.current.focus();
        }}
        className={cn(
          "fixed top-1/2 left-1/2 z-50 -translate-x-1/2 -translate-y-1/2 overflow-auto rounded-none bg-white shadow-float",
          "w-[min(560px,calc(100vw-32px))] max-h-[calc(100vh-48px)] focus:outline-none",
          "data-[state=open]:animate-pop data-[state=closed]:animate-out data-[state=closed]:fade-out-0",
          className,
        )}
        {...props}
      >
        {children}
      </DialogPrimitive.Content>
    </DialogPortal>
  );
}

export type DialogHeaderProps = ComponentProps<"div"> & { showClose?: boolean };

export function DialogHeader({ className, children, showClose = true, ...props }: DialogHeaderProps) {
  return (
    <div
      data-slot="dialog-header"
      className={cn("flex items-center justify-between gap-3 border-b border-line px-6 py-4", className)}
      {...props}
    >
      <div className="min-w-0">{children}</div>
      {showClose ? (
        <DialogPrimitive.Close asChild>
          <IconButton aria-label="Đóng" className="-my-2 -mr-3">
            <X aria-hidden="true" />
          </IconButton>
        </DialogPrimitive.Close>
      ) : null}
    </div>
  );
}

export function DialogBody({ className, ...props }: ComponentProps<"div">) {
  return <div data-slot="dialog-body" className={cn("grid gap-4 p-6", className)} {...props} />;
}

export function DialogFooter({ className, ...props }: ComponentProps<"div">) {
  return (
    <div
      data-slot="dialog-footer"
      className={cn("flex flex-wrap justify-end gap-2 border-t border-line px-6 py-4", className)}
      {...props}
    />
  );
}

export function DialogTitle({ className, ...props }: ComponentProps<typeof DialogPrimitive.Title>) {
  return <DialogPrimitive.Title data-slot="dialog-title" className={cn("text-lg font-semibold text-ink", className)} {...props} />;
}

export function DialogDescription({ className, ...props }: ComponentProps<typeof DialogPrimitive.Description>) {
  return (
    <DialogPrimitive.Description data-slot="dialog-description" className={cn("text-ink-2", className)} {...props} />
  );
}
