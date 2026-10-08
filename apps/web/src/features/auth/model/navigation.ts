export const LOGIN_PATH = "/login";
export const FIRST_LOGIN_PATH = "/first-login";

/**
 * Post-login destination from `?next=`, accepted only as an in-app absolute path:
 * one leading slash, no protocol-relative `//`, no backslash (browsers treat `/\` as `//`),
 * no control characters. Anything else falls back to the role's home.
 */
export function safeNext(next: string | null | undefined): string | undefined {
  if (!next?.startsWith("/")) return undefined;
  if (next.startsWith("//") || next.includes("\\")) return undefined;
  // eslint-disable-next-line no-control-regex -- rejecting control characters is the point
  if (/[\u0000-\u001f\u007f]/.test(next)) return undefined;
  return next;
}

/** `/login?next=<path>` so the user lands back where the session ran out. */
export function loginUrl(next?: string): string {
  const target = safeNext(next);
  if (!target || target === "/" || target.startsWith(LOGIN_PATH)) return LOGIN_PATH;
  return `${LOGIN_PATH}?${new URLSearchParams({ next: target }).toString()}`;
}

/** Reads `token` from a URL fragment such as `#token=abc`. */
export function tokenFromHash(hash: string): string | null {
  const token = new URLSearchParams(hash.replace(/^#/, "")).get("token");
  return token && token.trim() !== "" ? token : null;
}
