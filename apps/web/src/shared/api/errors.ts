/** Copy shown when the server cannot be reached or answers without the error envelope. */
export const NETWORK_ERROR_MESSAGE = "Không kết nối được máy chủ. Thử lại sau.";

/** Error raised by `http()` for every non-2xx response, network failure or invalid payload. */
export class ApiError extends Error {
  readonly status: number;
  readonly code: string;
  readonly details: unknown;

  constructor(status: number, code: string, message: string, details?: unknown) {
    super(message);
    this.name = "ApiError";
    this.status = status;
    this.code = code;
    this.details = details;
  }
}

export function isApiError(error: unknown): error is ApiError {
  return error instanceof ApiError;
}

/** 4xx responses are final: retrying cannot change the answer. Network errors use status 0. */
export function isClientError(error: unknown): boolean {
  return isApiError(error) && error.status >= 400 && error.status < 500;
}

export function hasCode(error: unknown, code: string): boolean {
  return isApiError(error) && error.code === code;
}
