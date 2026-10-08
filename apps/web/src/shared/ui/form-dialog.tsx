import { zodResolver } from "@hookform/resolvers/zod";
import { useEffect, useRef, useState, type ReactNode } from "react";
import { useForm, type DefaultValues, type FieldValues, type UseFormReturn } from "react-hook-form";
import type { z } from "zod";
import { NETWORK_ERROR_MESSAGE, isApiError } from "@/shared/api/errors";
import { Alert } from "./alert";
import { Button } from "./button";
import { Dialog, DialogBody, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "./dialog";

export interface FormDialogProps<TIn extends FieldValues, TOut extends FieldValues> {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  title: string;
  description?: ReactNode;
  schema: z.ZodType<TOut, TIn>;
  defaultValues: DefaultValues<TIn>;
  submitLabel: string;
  cancelLabel?: string;
  /** Throw (e.g. an ApiError) to keep the dialog open and show the message; resolve to close it. */
  onSubmit: (values: TOut) => Promise<unknown>;
  children: (form: UseFormReturn<TIn, unknown, TOut>) => ReactNode;
}

/**
 * Modal form on react-hook-form + zod. Field errors render next to fields (via `Field`);
 * a failed submit shows the server message in a danger alert. The first input gets focus on open.
 */
export function FormDialog<TIn extends FieldValues, TOut extends FieldValues>({
  open,
  onOpenChange,
  title,
  description,
  schema,
  defaultValues,
  submitLabel,
  cancelLabel = "Hủy",
  onSubmit,
  children,
}: FormDialogProps<TIn, TOut>) {
  const form = useForm<TIn, unknown, TOut>({ resolver: zodResolver(schema), defaultValues });
  const [formError, setFormError] = useState<string | null>(null);
  const contentRef = useRef<HTMLDivElement>(null);
  const { reset } = form;
  const submitting = form.formState.isSubmitting;

  // A stale server error from the previous opening must not reappear.
  const [wasOpen, setWasOpen] = useState(open);
  if (open !== wasOpen) {
    setWasOpen(open);
    if (open) setFormError(null);
  }

  useEffect(() => {
    if (open) reset(defaultValues);
    // Reset only when the dialog opens; defaultValues identity changes on every parent render.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [open, reset]);

  const submit = form.handleSubmit(async (values) => {
    setFormError(null);
    try {
      await onSubmit(values);
      onOpenChange(false);
    } catch (error) {
      setFormError(isApiError(error) ? error.message : NETWORK_ERROR_MESSAGE);
    }
  });

  return (
    <Dialog open={open} onOpenChange={(next) => {
        if (!submitting) onOpenChange(next);
      }}>
      <DialogContent
        ref={contentRef}
        {...(description ? {} : { "aria-describedby": undefined })}
        onOpenAutoFocus={(event) => {
          const first = contentRef.current?.querySelector<HTMLElement>(
            "input:not([type=hidden]):not([disabled]), textarea:not([disabled]), select:not([disabled]), button[role=combobox]",
          );
          if (first) {
            event.preventDefault();
            first.focus();
          }
        }}
      >
        <form noValidate onSubmit={(e) => void submit(e)}>
          <DialogHeader showClose={!submitting}>
            <DialogTitle>{title}</DialogTitle>
            {description ? <DialogDescription className="mt-1 text-sm text-ink-3">{description}</DialogDescription> : null}
          </DialogHeader>
          <DialogBody>
            {children(form)}
            {formError ? (
              <Alert variant="danger" role="alert">
                {formError}
              </Alert>
            ) : null}
          </DialogBody>
          <DialogFooter>
            <Button
              variant="outline"
              disabled={submitting}
              onClick={() => {
                onOpenChange(false);
              }}
            >
              {cancelLabel}
            </Button>
            <Button type="submit" disabled={submitting} aria-busy={submitting}>
              {submitLabel}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
