import { queryOptions } from "@tanstack/react-query";
import { z } from "zod";
import { isApiError } from "@/shared/api/errors";
import { http } from "@/shared/api/http";
import { userSchema, type User } from "@/shared/api/schemas";

export interface LoginInput {
  email: string;
  password: string;
}

export interface ChangePasswordInput {
  /** Omitted on first login: the temporary password was just used to sign in. */
  currentPassword?: string;
  newPassword: string;
  confirmPassword: string;
}

export interface ForgotPasswordInput {
  email: string;
}

export interface ResetPasswordInput {
  token: string;
  newPassword: string;
  confirmPassword: string;
}

const loginResponseSchema = z.object({ user: userSchema });

export const authApi = {
  /** `POST /auth/login` → the signed-in user; the session cookie is set by the server. */
  login: async (input: LoginInput): Promise<User> => {
    const { user } = await http("/auth/login", { method: "POST", body: input, schema: loginResponseSchema });
    return user;
  },
  logout: (): Promise<undefined> => http("/auth/logout", { method: "POST" }),
  me: (signal?: AbortSignal): Promise<User> => http("/auth/me", { schema: userSchema, signal }),
  changePassword: (input: ChangePasswordInput): Promise<User> =>
    http("/auth/change-password", { method: "POST", body: input, schema: userSchema }),
  /** Always 202, whatever the email, so the response never reveals whether an account exists. */
  forgotPassword: (input: ForgotPasswordInput): Promise<undefined> =>
    http("/auth/forgot-password", { method: "POST", body: input }),
  resetPassword: (input: ResetPasswordInput): Promise<undefined> =>
    http("/auth/reset-password", { method: "POST", body: input }),
};

/** Query key of the current user; `null` data means "signed out". */
export const meKey = ["auth", "me"] as const;

/** Current user, or `null` when the session is missing or expired (401 is an answer, not an error). */
export async function fetchMe({ signal }: { signal?: AbortSignal } = {}): Promise<User | null> {
  try {
    return await authApi.me(signal);
  } catch (error) {
    if (isApiError(error) && error.status === 401) return null;
    throw error;
  }
}

export const meQuery = queryOptions({ queryKey: meKey, queryFn: fetchMe });
