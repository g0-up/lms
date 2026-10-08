import { defineConfig, mergeConfig } from "vitest/config";
import viteConfig from "./vite.config.ts";

// Dates render in the users' time zone; pin it so date tests do not depend on the host.
process.env.TZ = "Asia/Ho_Chi_Minh";

export default defineConfig((configEnv) =>
  mergeConfig(viteConfig(configEnv), {
    test: {
      environment: "jsdom",
      setupFiles: ["./src/shared/test/setup.ts"],
      include: ["src/**/*.test.{ts,tsx}"],
      restoreMocks: true,
    },
  }),
);
