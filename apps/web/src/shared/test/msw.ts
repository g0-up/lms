import { http, HttpResponse } from "msw";
import { setupServer } from "msw/node";
import type { User } from "@/shared/api/schemas";

/** Absolute API path as the app requests it (`/api/v1` + path). */
export const api = (path: string) => `/api/v1${path}`;

/** Seed-like users matching the API's `MeDTO`. */
export const users = {
  admin: {
    id: "u-admin",
    name: "Trần Minh Quân",
    email: "quan.tran@goup.vn",
    role: "admin",
    status: "active",
    mustChangePassword: false,
  },
  teacher: {
    id: "u-teacher",
    name: "Lê Thu Hương",
    email: "huong.le@goup.vn",
    role: "teacher",
    status: "active",
    mustChangePassword: false,
  },
  student: {
    id: "u-student",
    name: "Nguyễn Hoàng An",
    email: "an.nguyen@gmail.com",
    role: "student",
    status: "active",
    mustChangePassword: false,
  },
  invited: {
    id: "u-invited",
    name: "Bùi Quang Minh",
    email: "minh.bui@gmail.com",
    role: "student",
    status: "invited",
    mustChangePassword: true,
    tempPasswordExpiresAt: "2026-10-08T09:30:00+07:00",
  },
} satisfies Record<string, User>;

/** The API's error envelope. */
export function apiError(status: number, code: string, message: string) {
  return HttpResponse.json({ error: { code, message } }, { status });
}

export const unauthenticated = () => apiError(401, "UNAUTHENTICATED", "Vui lòng đăng nhập");

/** In-memory session behind the default auth handlers. */
let session: User | null = null;

export function setSession(user: User | null): void {
  session = user;
}

/** Happy-path auth API: login accepts any password for a known email. */
export const authHandlers = [
  http.get(api("/auth/me"), () => (session ? HttpResponse.json(session) : unauthenticated())),
  http.post(api("/auth/login"), async ({ request }) => {
    const body = (await request.json()) as { email?: string };
    const user = Object.values(users).find((u) => u.email === body.email);
    if (!user) return apiError(401, "UNAUTHENTICATED", "Email hoặc mật khẩu không đúng.");
    session = user;
    return HttpResponse.json({ user });
  }),
  http.post(api("/auth/logout"), () => {
    session = null;
    return new HttpResponse(null, { status: 204 });
  }),
  http.post(api("/auth/forgot-password"), () =>
    HttpResponse.json({ message: "Nếu email tồn tại, chúng tôi đã gửi hướng dẫn đặt lại mật khẩu." }, { status: 202 }),
  ),
  http.post(api("/auth/reset-password"), () => new HttpResponse(null, { status: 204 })),
  http.post(api("/auth/change-password"), () => {
    if (!session) return unauthenticated();
    session = { ...session, status: "active", mustChangePassword: false, tempPasswordExpiresAt: undefined };
    return HttpResponse.json(session);
  }),
];

export const server = setupServer(...authHandlers);
