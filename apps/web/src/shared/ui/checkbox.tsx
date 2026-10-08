import { Check } from "lucide-react";
import { Checkbox as CheckboxPrimitive } from "radix-ui";
import type { ComponentProps } from "react";
import { cn } from "@/shared/lib/cn";

/** 18px box; wrap it with its text in a `label` of at least 44px height for the hit area. */
export function Checkbox({ className, ...props }: ComponentProps<typeof CheckboxPrimitive.Root>) {
  return (
    <CheckboxPrimitive.Root
      data-slot="checkbox"
      className={cn(
        // design-audit-allow hit-area: the wrapping label provides the 44px target.
        "peer grid size-[18px] shrink-0 cursor-pointer place-items-center rounded-xs border border-gray-300 bg-white",
        "data-[state=checked]:border-navy-700 data-[state=checked]:bg-navy-700 data-[state=checked]:text-white",
        "aria-invalid:border-destructive disabled:cursor-not-allowed disabled:opacity-45",
        className,
      )}
      {...props}
    >
      <CheckboxPrimitive.Indicator data-slot="checkbox-indicator" className="grid place-items-center">
        <Check className="size-3.5" strokeWidth={3} aria-hidden="true" />
      </CheckboxPrimitive.Indicator>
    </CheckboxPrimitive.Root>
  );
}
