import type { z } from "zod";
import { ApiError, NETWORK_ERROR_MESSAGE } from "./errors";
import { errorEnvelopeSchema } from "./schemas";

export const API_BASE = "/api/v1";

export type HttpMethod = "GET" | "POST" | "PUT" | "PATCH" | "DELETE";

export interface HttpOptions<S extends z.ZodType | undefined = undefined> {
  method?: HttpMethod;
  body?: unknown;
  schema?: S;
  signal?: AbortSignal;
}

type Result<S> = S extends z.ZodType ? z.output<S> : undefined;

/**
 * Same-origin JSON client for the LMS API.
 *
 * - Sends the session cookie (`credentials: same-origin`) and always sets `X-Requested-With: fetch`,
 *   which the API checks on every request alongside its cross-origin protection.
 * - Non-2xx responses become `ApiError(status, code, message, details)` from the error envelope.
 * - 204 resolves to `undefined`; otherwise the body is validated with `schema` when given.
 * - Never redirects: navigation on 401/403 is decided by the query client.
 */
export async function http<S extends z.ZodType | undefined = undefined>(
  path: string,
  options: HttpOptions<S> = {},
): Promise<Result<S>> {
  const { method = "GET", body, schema, signal } = options;
  const headers: Record<string, string> = { "X-Requested-With": "fetch", Accept: "application/json" };
  if (body !== undefined) headers["Content-Type"] = "application/json";

  let response: Response;
  try {
    response = await fetch(API_BASE + path, {
      method,
      credentials: "same-origin",
      headers,
      body: body === undefined ? undefined : JSON.stringify(body),
      signal,
    });
  } catch (cause) {
    // A cancelled request (React Query aborts on unmount/key change) is not a connection problem.
    // Checked via the signal: the abort error's DOMException class differs between environments.
    if (signal?.aborted) throw cause;
    throw new ApiError(0, "NETWORK_ERROR", NETWORK_ERROR_MESSAGE, cause);
  }

  if (!response.ok) throw await toApiError(response);

  if (response.status === 204 || schema === undefined) {
    return undefined as Result<S>;
  }

  let json: unknown;
  try {
    json = await response.json();
  } catch (cause) {
    throw new ApiError(response.status, "INVALID_RESPONSE", "Phản hồi từ máy chủ không hợp lệ.", cause);
  }

  const parsed = schema.safeParse(json);
  if (!parsed.success) {
    throw new ApiError(
      response.status,
      "INVALID_RESPONSE",
      `Phản hồi từ máy chủ không hợp lệ (${method} ${path}).`,
      parsed.error.issues,
    );
  }
  return parsed.data as Result<S>;
}

async function toApiError(response: Response): Promise<ApiError> {
  let json: unknown = null;
  try {
    json = await response.json();
  } catch {
    // Non-JSON error body (proxy page, empty 502): fall through to the generic copy.
  }
  const envelope = errorEnvelopeSchema.safeParse(json);
  if (envelope.success) {
    const { code, message, details } = envelope.data.error;
    return new ApiError(response.status, code, message, details);
  }
  return new ApiError(response.status, `HTTP_${String(response.status)}`, NETWORK_ERROR_MESSAGE);
}
