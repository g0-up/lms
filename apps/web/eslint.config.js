import js from "@eslint/js";
import { defineConfig, globalIgnores } from "eslint/config";
import reactHooks from "eslint-plugin-react-hooks";
import reactRefresh from "eslint-plugin-react-refresh";
import globals from "globals";
import tseslint from "typescript-eslint";

// Feature chỉ được dùng qua public API `index.ts` của nó; lớp phụ thuộc một chiều shared → features → app.
const BASE_PATHS = [
  { name: "react-router-dom", message: "React Router v8: import từ 'react-router' (RouterProvider từ 'react-router/dom')." },
];
const BASE_PATTERNS = [
  { group: ["@/features/*/*"], message: "Import feature qua index.ts: @/features/<tên>." },
  { group: ["../../features", "../../features/*"], message: "Dùng alias @/features/<tên> thay vì đường dẫn tương đối." },
];

function restrictedImports({ paths = [], patterns = [] } = {}) {
  return { paths: [...BASE_PATHS, ...paths], patterns: [...BASE_PATTERNS, ...patterns] };
}

const NO_APP = { group: ["@/app", "@/app/*"], message: "Chỉ app/ được import app/." };
const NO_FEATURES = { group: ["@/features", "@/features/*"], message: "shared/ không phụ thuộc feature." };

export default defineConfig(
  globalIgnores(["dist", "coverage", "playwright-report", "test-results", "blob-report"]),
  {
    files: ["**/*.{ts,tsx}"],
    extends: [
      js.configs.recommended,
      tseslint.configs.strictTypeChecked,
      reactHooks.configs.flat.recommended,
      reactRefresh.configs.vite,
    ],
    languageOptions: {
      ecmaVersion: 2022,
      globals: globals.browser,
      parserOptions: {
        projectService: true,
        tsconfigRootDir: import.meta.dirname,
      },
    },
    rules: {
      "@typescript-eslint/no-unused-vars": ["error", { argsIgnorePattern: "^_", varsIgnorePattern: "^_" }],
      "no-restricted-imports": ["error", restrictedImports()],
      // React Router dùng `throw redirect(...)` (một Response) để điều hướng trong middleware/loader.
      "@typescript-eslint/only-throw-error": ["error", { allow: [{ from: "lib", name: "Response" }] }],
    },
  },
  {
    files: ["src/shared/**/*.{ts,tsx}"],
    rules: { "no-restricted-imports": ["error", restrictedImports({ patterns: [NO_APP, NO_FEATURES] })] },
  },
  {
    files: ["src/features/**/*.{ts,tsx}"],
    rules: { "no-restricted-imports": ["error", restrictedImports({ patterns: [NO_APP] })] },
  },
  {
    // model/ là logic thuần (schema, điều hướng, quy tắc): không React, không gọi mạng.
    files: ["src/features/*/model/**/*.{ts,tsx}"],
    rules: {
      "no-restricted-imports": [
        "error",
        restrictedImports({
          patterns: [
            NO_APP,
            { group: ["react", "react-dom", "react-dom/*", "react-router", "@tanstack/*"], message: "model/ không dùng React." },
            { group: ["@/shared/api/http", "../api", "../api/*"], message: "model/ không gọi mạng." },
          ],
        }),
      ],
      "no-restricted-globals": ["error", { name: "fetch", message: "model/ không gọi mạng." }],
    },
  },
  {
    // shadcn export cả variant; app/ export route kèm component.
    files: ["src/shared/ui/**/*.tsx", "src/shared/test/**/*.tsx", "src/app/**/*.tsx", "src/features/*/routes.tsx"],
    rules: { "react-refresh/only-export-components": "off" },
  },
  {
    // Playwright fixture dùng tham số `use`, không phải React hook.
    files: ["e2e/**/*.ts", "*.config.ts"],
    languageOptions: { globals: globals.node },
    rules: { "react-hooks/rules-of-hooks": "off" },
  },
  {
    files: ["**/*.{js,mjs}"],
    extends: [js.configs.recommended],
    languageOptions: { globals: globals.node },
  },
);
