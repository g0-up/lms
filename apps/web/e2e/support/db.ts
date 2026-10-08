import pg from "pg";

// Truy vấn DB chỉ để đọc kết quả (SELECT/WITH). Ngoại lệ duy nhất: expireResetToken cho kịch bản liên kết hết hạn.
const url =
  process.env.E2E_DATABASE_URL ?? `postgres://lms:lms@localhost:${process.env.POSTGRES_HOST_PORT ?? "5432"}/lms`;

let pool: pg.Pool | undefined;

function getPool(): pg.Pool {
  pool ??= new pg.Pool({ connectionString: url, max: 2 });
  return pool;
}

const READ_ONLY = /^\s*(select|with)\b/i;
const WRITE_WORD = /\b(insert|update|delete|truncate|alter|drop|create|grant)\b/i;

export async function query<T extends pg.QueryResultRow>(sql: string, params: unknown[] = []): Promise<T[]> {
  if (!READ_ONLY.test(sql) || WRITE_WORD.test(sql)) throw new Error("db.ts chỉ cho phép truy vấn đọc");
  const res = await getPool().query<T>(sql, params);
  return res.rows;
}

export async function one<T extends pg.QueryResultRow>(sql: string, params: unknown[] = []): Promise<T> {
  const rows = await query<T>(sql, params);
  if (rows.length !== 1) throw new Error(`Cần đúng 1 dòng, nhận ${String(rows.length)}`);
  return rows[0];
}

export async function count(sql: string, params: unknown[] = []): Promise<number> {
  const row = await one<{ n: string }>(sql, params);
  return Number(row.n);
}

/** Đưa token đặt lại mật khẩu còn hiệu lực của email về quá khứ (ngoại lệ ghi duy nhất, có trong kế hoạch). */
export async function expireResetToken(email: string): Promise<number> {
  const res = await getPool().query(
    `UPDATE password_reset_tokens SET expires_at = now() - interval '1 minute'
      WHERE used_at IS NULL AND user_id = (SELECT id FROM users WHERE email_normalized = lower($1))`,
    [email],
  );
  return res.rowCount ?? 0;
}

export const ids = {
  classId: async (code: string) => (await one<{ id: string }>("SELECT id FROM classes WHERE code = $1", [code])).id,
  stageId: async (code: string) => (await one<{ id: string }>("SELECT id FROM stages WHERE code = $1", [code])).id,
  courseId: async (code: string) => (await one<{ id: string }>("SELECT id FROM courses WHERE code = $1", [code])).id,
  userId: async (email: string) =>
    (await one<{ id: string }>("SELECT id FROM users WHERE email_normalized = lower($1)", [email])).id,
  stageVersionId: async (stageCode: string, no: number) =>
    (
      await one<{ id: string }>(
        "SELECT sv.id FROM stage_versions sv JOIN stages s ON s.id = sv.stage_id WHERE s.code = $1 AND sv.version_no = $2",
        [stageCode, no],
      )
    ).id,
  courseVersionId: async (courseCode: string, no: number) =>
    (
      await one<{ id: string }>(
        "SELECT cv.id FROM course_versions cv JOIN courses c ON c.id = cv.course_id WHERE c.code = $1 AND cv.version_no = $2",
        [courseCode, no],
      )
    ).id,
  /** Học liệu theo lesson_key trong phiên bản chặng đang được lớp dùng. */
  lessonInClass: async (classCode: string, lessonKey: string) =>
    (
      await one<{ id: string }>(
        `SELECT l.id FROM classes c
           JOIN course_version_stages cvs ON cvs.course_version_id = c.course_version_id
           JOIN lessons l ON l.stage_version_id = cvs.stage_version_id
          WHERE c.code = $1 AND l.lesson_key = $2`,
        [classCode, lessonKey],
      )
    ).id,
  memberId: async (classCode: string, email: string) =>
    (
      await one<{ id: string }>(
        `SELECT m.id FROM class_members m JOIN classes c ON c.id = m.class_id JOIN users u ON u.id = m.user_id
          WHERE c.code = $1 AND u.email_normalized = lower($2)`,
        [classCode, email],
      )
    ).id,
};

export async function closeDb(): Promise<void> {
  await pool?.end();
  pool = undefined;
}
