import { Check } from "lucide-react";
import { DropdownMenu as DropdownMenuPrimitive } from "radix-ui";
import type { ComponentProps } from "react";
import { cn } from "@/shared/lib/cn";

export const DropdownMenu = DropdownMenuPrimitive.Root;
export const DropdownMenuTrigger = DropdownMenuPrimitive.Trigger;
export const DropdownMenuGroup = DropdownMenuPrimitive.Group;
export const DropdownMenuPortal = DropdownMenuPrimitive.Portal;
export const DropdownMenuRadioGroup = DropdownMenuPrimitive.RadioGroup;

export function DropdownMenuContent({
  className,
  sideOffset = 4,
  ...props
}: ComponentProps<typeof DropdownMenuPrimitive.Content>) {
  return (
    <DropdownMenuPrimitive.Portal>
      <DropdownMenuPrimitive.Content
        data-slot="dropdown-menu-content"
        sideOffset={sideOffset}
        className={cn(
          "z-50 min-w-48 overflow-hidden rounded-sm bg-white p-1 text-ink-2 shadow-float",
          "data-[state=open]:animate-in data-[state=open]:fade-in-0 data-[state=closed]:animate-out data-[state=closed]:fade-out-0",
          className,
        )}
        {...props}
      />
    </DropdownMenuPrimitive.Portal>
  );
}

const itemClass = [
  "relative flex min-h-11 cursor-pointer items-center gap-2 rounded-xs px-3 py-2 text-sm outline-none select-none",
  "focus:bg-surface-blue-50 focus:text-navy-700 data-[disabled]:pointer-events-none data-[disabled]:opacity-45",
  "[&_svg]:size-4 [&_svg]:shrink-0",
].join(" ");

export type DropdownMenuItemProps = ComponentProps<typeof DropdownMenuPrimitive.Item> & {
  variant?: "default" | "destructive";
};

export function DropdownMenuItem({ className, variant = "default", ...props }: DropdownMenuItemProps) {
  return (
    <DropdownMenuPrimitive.Item
      data-slot="dropdown-menu-item"
      className={cn(itemClass, variant === "destructive" && "text-danger focus:bg-danger-bg focus:text-danger", className)}
      {...props}
    />
  );
}

export function DropdownMenuRadioItem({
  className,
  children,
  ...props
}: ComponentProps<typeof DropdownMenuPrimitive.RadioItem>) {
  return (
    <DropdownMenuPrimitive.RadioItem data-slot="dropdown-menu-radio-item" className={cn(itemClass, "pl-8", className)} {...props}>
      <span className="absolute left-2 grid size-4 place-items-center">
        <DropdownMenuPrimitive.ItemIndicator>
          <Check aria-hidden="true" />
        </DropdownMenuPrimitive.ItemIndicator>
      </span>
      {children}
    </DropdownMenuPrimitive.RadioItem>
  );
}

export function DropdownMenuLabel({ className, ...props }: ComponentProps<typeof DropdownMenuPrimitive.Label>) {
  return (
    <DropdownMenuPrimitive.Label
      data-slot="dropdown-menu-label"
      className={cn("px-3 py-2 text-xs font-medium tracking-[1px] text-ink-3 uppercase", className)}
      {...props}
    />
  );
}

export function DropdownMenuSeparator({ className, ...props }: ComponentProps<typeof DropdownMenuPrimitive.Separator>) {
  return (
    <DropdownMenuPrimitive.Separator
      data-slot="dropdown-menu-separator"
      className={cn("-mx-1 my-1 h-px bg-line", className)}
      {...props}
    />
  );
}
