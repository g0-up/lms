import { useMemo, useRef, useState } from "react";
import { useWatch, type UseFormReturn } from "react-hook-form";
import { hasCode } from "@/shared/api/errors";
import { Field } from "@/shared/ui/field";
import { FormDialog } from "@/shared/ui/form-dialog";
import { Input } from "@/shared/ui/input";
import { codeNameSchema, normalizeCode, type CodeNameOutput, type CodeNameValues } from "../model/code-name-form";

export interface CodeNameDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  title: string;
  submitLabel: string;
  codeLabel: string;
  codePlaceholder: string;
  codeHelp?: string;
  nameLabel: string;
  namePlaceholder: string;
  note: string;
  requiredMessage: string;
  /** Field error shown while the code input still holds a code the server refused as taken. */
  conflictMessage: string;
  /** Throw an ApiError to keep the dialog open; CONFLICT becomes the code field's error. */
  onCreate: (values: CodeNameOutput) => Promise<unknown>;
}

const DEFAULTS: CodeNameValues = { code: "", name: "" };

/** Create dialog for a stage or a course: code and name, created with an empty draft v1. */
export function CodeNameDialog({ open, onOpenChange, onCreate, requiredMessage, conflictMessage, ...copy }: CodeNameDialogProps) {
  const schema = useMemo(() => codeNameSchema(requiredMessage), [requiredMessage]);
  const [takenCode, setTakenCode] = useState<string | null>(null);
  // FormDialog closes once onSubmit resolves; a CONFLICT must keep it open with a field error instead.
  const keepOpen = useRef(false);

  return (
    <FormDialog
      open={open}
      onOpenChange={(next) => {
        if (!next && keepOpen.current) {
          keepOpen.current = false;
          return;
        }
        onOpenChange(next);
      }}
      title={copy.title}
      schema={schema}
      defaultValues={DEFAULTS}
      submitLabel={copy.submitLabel}
      onSubmit={async (values) => {
        setTakenCode(null);
        try {
          await onCreate(values);
        } catch (error) {
          if (!hasCode(error, "CONFLICT")) throw error;
          setTakenCode(values.code);
          keepOpen.current = true;
        }
      }}
    >
      {(form) => <CodeNameFields form={form} takenCode={takenCode} conflictMessage={conflictMessage} {...copy} />}
    </FormDialog>
  );
}

type CodeNameFieldsProps = Pick<
  CodeNameDialogProps,
  "codeLabel" | "codePlaceholder" | "codeHelp" | "nameLabel" | "namePlaceholder" | "note" | "conflictMessage"
> & {
  form: UseFormReturn<CodeNameValues, unknown, CodeNameOutput>;
  takenCode: string | null;
};

function CodeNameFields({ form, takenCode, conflictMessage, ...copy }: CodeNameFieldsProps) {
  const code = useWatch({ control: form.control, name: "code" });
  const { errors } = form.formState;
  const conflict = takenCode !== null && normalizeCode(code) === takenCode ? conflictMessage : undefined;
  return (
    <>
      <div className="grid grid-cols-2 gap-4 max-[720px]:grid-cols-1">
        <Field label={copy.codeLabel} required help={copy.codeHelp} error={errors.code?.message ?? conflict}>
          <Input required placeholder={copy.codePlaceholder} autoComplete="off" {...form.register("code")} />
        </Field>
        <Field label={copy.nameLabel} required error={errors.name?.message}>
          <Input required placeholder={copy.namePlaceholder} autoComplete="off" {...form.register("name")} />
        </Field>
      </div>
      <p className="text-sm text-ink-3">{copy.note}</p>
    </>
  );
}
