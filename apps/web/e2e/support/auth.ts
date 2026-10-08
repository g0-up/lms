import { execFileSync } from "node:child_process";
import { mkdirSync, readFileSync } from "node:fs";
import path from "node:path";
import { request, type APIRequestContext, type Browser, type BrowserContext } from "@playwright/test";

// Gốc repo: apps/web/e2e/support → ../../../..
export const REPO_ROOT = path.resolve(import.meta.dirname, "../../../..");
export const BASE_URL = process.env.E2E_BASE_URL ?? "http://localhost:8081";
const AUTH_DIR = path.resolve(import.meta.dirname, "../.auth");
const FIXTURE_DIR = path.resolve(import.meta.dirname, "../fixtures");

export type Role = "admin" | "teacher" | "teacher2" | "student";
export const ROLES: readonly Role[] = ["admin", "teacher", "teacher2", "student"];

// Header bắt buộc cho mọi request ghi (chống CSRF ở API).
export const FETCH_HEADERS = { "X-Requested-With": "fetch" } as const;

let cachedPassword: string | undefined;

/**
 * Mật khẩu chung của tài khoản seed: env SEED_PASSWORD, nếu không có thì đọc env của service api trong
 * compose profile "full" (`docker compose config`; image api không có shell). Không bao giờ in giá trị này.
 */
export function seedPassword(): string {
  if (cachedPassword) return cachedPassword;
  let value = process.env.SEED_PASSWORD ?? "";
  if (!value) {
    const config = execFileSync(
      "docker",
      ["compose", "-f", "infra/docker-compose.yml", "--profile", "full", "config", "--format", "json"],
      { cwd: REPO_ROOT, encoding: "utf8", stdio: ["ignore", "pipe", "ignore"] },
    );
    const parsed = JSON.parse(config) as { services?: { api?: { environment?: Record<string, string | null> } } };
    value = parsed.services?.api?.environment?.SEED_PASSWORD ?? "";
  }
  if (!value) throw new Error("Thiếu SEED_PASSWORD (env hoặc container api của profile full)");
  cachedPassword = value;
  return value;
}

/** Body đăng nhập của một vai trò, đọc từ fixture (không chứa mật khẩu thật) rồi thay ${SEED_PASSWORD}. */
export function loginBody(role: Role): { email: string; password: string } {
  const raw = readFileSync(path.join(FIXTURE_DIR, `login-${role}.json`), "utf8");
  const parsed = JSON.parse(raw) as { email: string; password: string };
  return { email: parsed.email, password: parsed.password === "${SEED_PASSWORD}" ? seedPassword() : parsed.password };
}

export function emailOf(role: Role): string {
  return loginBody(role).email;
}

export function storageStatePath(role: Role): string {
  return path.join(AUTH_DIR, `${role}.json`);
}

/** Đăng nhập qua API; ném lỗi kèm mã HTTP (không kèm body đăng nhập) nếu thất bại. */
export async function loginContext(email: string, password: string): Promise<APIRequestContext> {
  const ctx = await request.newContext({ baseURL: BASE_URL, extraHTTPHeaders: FETCH_HEADERS });
  const res = await ctx.post("/api/v1/auth/login", { data: { email, password } });
  if (!res.ok()) {
    await ctx.dispose();
    throw new Error(`Đăng nhập ${email} thất bại: HTTP ${String(res.status())}`);
  }
  return ctx;
}

/** Ghi storageState cho mọi vai trò; gọi lại sau mỗi seed-reset vì seed xóa toàn bộ phiên. */
export async function saveStorageStates(): Promise<void> {
  mkdirSync(AUTH_DIR, { recursive: true });
  for (const role of ROLES) {
    const { email, password } = loginBody(role);
    const ctx = await loginContext(email, password);
    await ctx.storageState({ path: storageStatePath(role) });
    await ctx.dispose();
  }
}

/** APIRequestContext đã đăng nhập theo vai trò (dùng storageState hiện tại). */
export async function apiAs(role: Role): Promise<APIRequestContext> {
  return request.newContext({ baseURL: BASE_URL, extraHTTPHeaders: FETCH_HEADERS, storageState: storageStatePath(role) });
}

/** Context trình duyệt đã đăng nhập theo vai trò. */
export async function browserAs(browser: Browser, role: Role): Promise<BrowserContext> {
  return browser.newContext({ storageState: storageStatePath(role) });
}
