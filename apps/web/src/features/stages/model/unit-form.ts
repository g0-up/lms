import { z } from "zod";

export const UNIT_TITLE_MAX = 120;
export const UNIT_TITLE_REQUIRED = "Nhập tên bài học.";
export const UNIT_TITLE_TOO_LONG = `Tên bài học tối đa ${String(UNIT_TITLE_MAX)} ký tự.`;

/** "Thêm bài học"/"Đổi tên bài học" values; the length counts characters after trimming, as the server does. */
export const unitFormSchema = z.object({
  title: z
    .string()
    .trim()
    .min(1, UNIT_TITLE_REQUIRED)
    .refine((title) => Array.from(title).length <= UNIT_TITLE_MAX, UNIT_TITLE_TOO_LONG),
});
export type UnitFormValues = z.input<typeof unitFormSchema>;
export type UnitFormOutput = z.output<typeof unitFormSchema>;
