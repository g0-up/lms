import type { QueryClient } from "@tanstack/react-query";
import { redirect, type MiddlewareFunction } from "react-router";
import { FIRST_LOGIN_PATH, loginUrl, meQuery } from "@/features/auth";
import type { User } from "@/shared/api/schemas";
import { homeOf, type Role } from "@/shared/domain";
import { userContext } from "@/shared/lib/route";

function landingOf(user: User): string {
  return user.mustChangePassword ? FIRST_LOGIN_PATH : homeOf(user.role);
}

/** Route guards bound to the app's query client (the current user lives in its cache). */
export function createMiddleware(queryClient: QueryClient) {
  // Cached answer when present (login/logout keep it current), otherwise one request.
  const currentUser = () => queryClient.query({ ...meQuery, staleTime: "static" });

  /** Requires a session; a pending temporary password confines the user to /first-login. */
  const authMiddleware: MiddlewareFunction = async ({ request, context }) => {
    const user = await currentUser();
    const url = new URL(request.url);
    if (!user) throw redirect(loginUrl(url.pathname + url.search));
    if (user.mustChangePassword && url.pathname !== FIRST_LOGIN_PATH) throw redirect(FIRST_LOGIN_PATH);
    context.set(userContext, user);
  };

  /** Only users of `role`; anyone else goes to their own home. Place after `authMiddleware`. */
  function requireRole(role: Role): MiddlewareFunction {
    return ({ context }) => {
      const user = context.get(userContext);
      if (user.role !== role) throw redirect(homeOf(user.role));
    };
  }

  /** /first-login is only for a pending temporary password. Place after `authMiddleware`. */
  const requirePasswordChange: MiddlewareFunction = ({ context }) => {
    const user = context.get(userContext);
    if (!user.mustChangePassword) throw redirect(homeOf(user.role));
  };

  /**
   * Login and forgot-password are for guests; a signed-in user goes to their landing page.
   * If the session check itself fails (API unreachable) the form still renders, and submitting
   * shows the connection error.
   */
  const guestMiddleware: MiddlewareFunction = async () => {
    let user: User | null;
    try {
      user = await currentUser();
    } catch {
      return;
    }
    if (user) throw redirect(landingOf(user));
  };

  /** `/` has no page of its own: send everyone to the right place. */
  const indexMiddleware: MiddlewareFunction = async () => {
    const user = await currentUser();
    throw redirect(user ? landingOf(user) : loginUrl());
  };

  return { authMiddleware, requireRole, requirePasswordChange, guestMiddleware, indexMiddleware };
}
