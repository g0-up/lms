import { cloneElement, useId, type ReactElement, type ReactNode } from "react";
import { cn } from "@/shared/lib/cn";
import { Label } from "./label";

interface ControlProps {
  id?: string;
  "aria-describedby"?: string;
  "aria-invalid"?: boolean | "true" | "false";
}

export interface FieldProps {
  label: string;
  /** Shows the crimson asterisk; also pass `required` to the control itself. */
  required?: boolean;
  help?: ReactNode;
  error?: string;
  /** A single control element; it receives `id`, `aria-describedby` and `aria-invalid`. */
  children: ReactElement<ControlProps>;
  className?: string;
}

/** Label, control, help and error wired together for assistive technology. */
export function Field({ label, required, help, error, children, className }: FieldProps) {
  const autoId = useId();
  const controlId = children.props.id ?? autoId;
  const helpId = help ? `${controlId}-help` : undefined;
  const errorId = error ? `${controlId}-error` : undefined;
  const describedBy = [children.props["aria-describedby"], helpId, errorId].filter(Boolean).join(" ") || undefined;

  return (
    <div data-slot="field" className={cn("grid gap-1.5", className)}>
      <Label htmlFor={controlId}>
        {label}
        {required ? (
          <span aria-hidden="true" className="ml-0.5 text-accent-crimson">
            *
          </span>
        ) : null}
      </Label>
      {cloneElement(children, {
        id: controlId,
        "aria-describedby": describedBy,
        "aria-invalid": error ? true : children.props["aria-invalid"],
      })}
      {help ? (
        <div id={helpId} className="text-xs text-ink-3">
          {help}
        </div>
      ) : null}
      {error ? (
        <div id={errorId} className="text-xs text-danger">
          {error}
        </div>
      ) : null}
    </div>
  );
}
