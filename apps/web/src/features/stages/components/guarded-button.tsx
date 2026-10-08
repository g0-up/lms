import { toast } from "sonner";
import { Button, type ButtonProps } from "@/shared/ui/button";

export type GuardedButtonProps = Omit<ButtonProps, "disabled" | "onClick" | "asChild"> & {
  /** Why the action is unavailable, or null. */
  blocker: string | null;
  onClick: () => void;
};

/**
 * Button that stays focusable while unavailable: `aria-disabled` plus the reason as `title`,
 * and a click only toasts that reason.
 */
export function GuardedButton({ blocker, onClick, title, ...props }: GuardedButtonProps) {
  return (
    <Button
      {...props}
      aria-disabled={blocker ? true : undefined}
      title={blocker ?? title}
      onClick={() => {
        if (blocker) toast.error(blocker);
        else onClick();
      }}
    />
  );
}
