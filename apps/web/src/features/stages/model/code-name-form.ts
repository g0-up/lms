import { z } from "zod";

/** "Tạo chặng"/"Tạo khóa học" values: both fields required, the code trimmed and uppercased. */
export function codeNameSchema(requiredMessage: string) {
  return z.object({
    code: z
      .string()
      .trim()
      .min(1, requiredMessage)
      .transform((code) => code.toUpperCase()),
    name: z.string().trim().min(1, requiredMessage),
  });
}

export type CodeNameValues = z.input<ReturnType<typeof codeNameSchema>>;
export type CodeNameOutput = z.output<ReturnType<typeof codeNameSchema>>;

/** The code as the server would store it, to compare against a code it refused. */
export const normalizeCode = (code: string) => code.trim().toUpperCase();
