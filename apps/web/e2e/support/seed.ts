import { execFileSync } from "node:child_process";
import { expect, type APIRequestContext } from "@playwright/test";
import { REPO_ROOT, saveStorageStates } from "./auth";
import { ids } from "./db";

/**
 * Seed lại DB bằng container api của profile "full" (`make e2e-seed-reset`) rồi đăng nhập lại mọi vai trò,
 * vì seed xóa toàn bộ phiên. Chỉ globalSetup và beforeAll/afterAll của describe @serial được gọi hàm này.
 */
export async function seedReset(): Promise<void> {
  try {
    execFileSync("make", ["--no-print-directory", "-C", REPO_ROOT, "e2e-seed-reset"], {
      stdio: ["ignore", "ignore", "pipe"],
      encoding: "utf8",
    });
  } catch (err) {
    const stderr = (err as { stderr?: string }).stderr ?? "";
    throw new Error(`make e2e-seed-reset thất bại: ${stderr.slice(-500)}`, { cause: err });
  }
  await saveStorageStates();
}

function isoDate(offsetDays: number): string {
  const d = new Date(Date.now() + offsetDays * 86_400_000);
  return d.toISOString().slice(0, 10);
}

export interface CreatedClass {
  id: string;
  code: string;
}

/** Tạo lớp nháp (mặc định BASIC v1, giảng viên Hương) qua API admin. */
export async function createClass(admin: APIRequestContext, code: string, courseVersionId?: string): Promise<CreatedClass> {
  const res = await admin.post("/api/v1/classes", {
    data: {
      code,
      name: `Lớp E2E ${code}`,
      courseVersionId: courseVersionId ?? (await ids.courseVersionId("BASIC", 1)),
      startDate: isoDate(-1),
      endDate: isoDate(30),
      teacherId: await ids.userId("huong.le@goup.vn"),
    },
  });
  expect(res.status(), await res.text()).toBe(201);
  const body = (await res.json()) as { id: string };
  return { id: body.id, code };
}

export async function invite(admin: APIRequestContext, classId: string, email: string, fullName = ""): Promise<void> {
  const res = await admin.post(`/api/v1/classes/${classId}/invitations`, { data: { email, fullName } });
  expect(res.status(), await res.text()).toBeLessThan(300);
}

export async function activateClass(admin: APIRequestContext, classId: string): Promise<void> {
  const res = await admin.post(`/api/v1/classes/${classId}/activate`);
  expect(res.status(), await res.text()).toBe(200);
}

export async function endClass(admin: APIRequestContext, classId: string): Promise<void> {
  const res = await admin.post(`/api/v1/classes/${classId}/end`);
  expect(res.status(), await res.text()).toBe(200);
}

/** Seed không có lớp `ended`: tạo lớp mới, (tùy chọn) mời học viên, kích hoạt rồi kết thúc. */
export async function createEndedClass(admin: APIRequestContext, code: string): Promise<CreatedClass> {
  const cls = await createClass(admin, code);
  await activateClass(admin, cls.id);
  await endClass(admin, cls.id);
  return cls;
}
