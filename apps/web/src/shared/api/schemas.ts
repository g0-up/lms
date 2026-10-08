import { z } from "zod";

/** ISO 8601 timestamp as sent by the API (RFC 3339, with offset). */
export const isoDate = z.iso.datetime({ offset: true });

export const roleSchema = z.enum(["admin", "teacher", "student"]);

export const userStatusSchema = z.enum(["invited", "active", "disabled"]);

/** `MeDTO` from `GET /auth/me` and `POST /auth/login`. */
export const userSchema = z.object({
  id: z.string().min(1),
  name: z.string(),
  email: z.string(),
  role: roleSchema,
  status: userStatusSchema,
  mustChangePassword: z.boolean(),
  tempPasswordExpiresAt: isoDate.nullish(),
});

export type User = z.infer<typeof userSchema>;

/** `{"error":{code,message,details}}`, the only error shape the API returns. */
export const errorEnvelopeSchema = z.object({
  error: z.object({
    code: z.string().min(1),
    message: z.string().min(1),
    details: z.unknown().optional(),
  }),
});
