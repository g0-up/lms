import { closeDb } from "./support/db";
import { seedReset } from "./support/seed";

// Seed lại đúng một lần cho cả lượt chạy rồi ghi storageState từng vai trò vào e2e/.auth/ (gitignore).
export default async function globalSetup(): Promise<void> {
  await seedReset();
  await closeDb();
}
