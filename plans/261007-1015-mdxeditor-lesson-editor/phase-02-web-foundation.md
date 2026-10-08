---
phase: 2
title: "Phase 2: Web foundation: dependency, upload media dùng chung, quy tắc markdown, client preview, prose"
status: completed
priority: P2
effort: "1 ngày"
dependencies: [1]
---

# Phase 2: Web foundation

## Goal

Chuẩn bị mọi mảnh không phải UI editor: cài MDXEditor + Lexical đúng phiên bản, tách luồng upload media dùng chung
cho video và ảnh, quy tắc markdown thuần (byte, ảnh, link) dùng cả ở editor lẫn khi submit, client gọi endpoint xem
trước, và class prose dùng chung để editor/xem trước/trang học viên trông giống nhau.

## Context & Requirements

- Phụ thuộc hợp đồng phase 1: `POST /api/v1/stages/markdown-preview {markdownSource} → 200 {html}`; 422 thông điệp
  "Ảnh trong markdown phải tải lên qua hệ thống."; golden `apps/api/internal/features/stages/testdata/markdown_preview.json`.
  Có thể làm song song phase 1 với fixture tạm trong nhánh, nhưng **phải** chuyển sang golden thật trước khi đóng phase.
- **Sở hữu file:** `apps/web/package.json`, `apps/web/pnpm-lock.yaml`, `apps/web/src/features/stages/**`,
  và hai thay đổi foundation có chủ đích (ghi rõ trong PR): `apps/web/src/shared/ui/markdown-content.tsx`
  (export class prose + style còn thiếu) và `apps/web/src/shared/ui/tabs.tsx` (mới, bọc Radix Tabs từ gói
  `radix-ui` đã có; dùng ở phase 4).
- Media: `kind: "image"` đã có ở API với `image/png|jpeg|webp|gif`, `MAX_IMAGE_BYTES` mặc định 10 MB
  (`apps/api/internal/features/media/entity.go:67`). Upload chỉ admin, `/media` rate limit 120 req/phút/người.
- `use-video-upload.ts` có `putToStorage` (XHR, chỉ header Content-Type, không credentials) và state machine
  `idle → uploading → done | error`.

## Files

| File | Thay đổi |
|---|---|
| `apps/web/package.json`, `pnpm-lock.yaml` | Thêm `@mdxeditor/editor` `~4.3.2`; `lexical`, `@lexical/list`, `@lexical/link`, `@lexical/rich-text` ghim **chính xác** phiên bản mà `@mdxeditor/editor@4.3.2` resolve. |
| `apps/web/src/features/stages/api/media-api.ts` | `UploadRequest.kind: "video" \| "image"`. |
| `apps/web/src/features/stages/api/upload-media.ts` (mới) | `putToStorage` chuyển từ hook sang; `uploadMedia(kind, file, {onProgress, signal}): Promise<Media>` gói 3 bước init → PUT → complete. |
| `apps/web/src/features/stages/hooks/use-video-upload.ts` | Dùng `uploadMedia("video", …)`; giữ nguyên state/hành vi/thông điệp. |
| `apps/web/src/features/stages/model/image-file.ts` (+ `.test.ts`) | `IMAGE_CONTENT_TYPES`, `MAX_IMAGE_BYTES = 10 * 1024 ** 2`, `imageFileError(file)`, `altFromFileName(name)`, `mediaContentUrl(id)`. |
| `apps/web/src/features/stages/model/markdown-rules.ts` (+ `.test.ts`) | `LESSON_BODY_MAX_BYTES`, `utf8Bytes`, `jsonBodyBytes`, `MEDIA_SRC`, `SAFE_URL`, `markdownSourceError(src)`, `lessonBodySizeError(body)`. |
| `apps/web/src/features/stages/api/stages-api.ts` | `previewMarkdown(markdownSource, signal)` → `{html}` qua `markdownPreviewSchema`. |
| `apps/web/src/features/stages/model/schemas.ts` | `markdownPreviewSchema = z.object({ html: z.string() })`. |
| `apps/web/src/features/stages/test/golden.ts` | Nạp `markdown_preview.json` qua schema; nạp `seed_markdown.json` (nguồn markdown của **cả 7** bài seed, golden do phase 1 sinh từ package `seed`) cho test round-trip ở phase 3. |
| `apps/web/src/features/stages/api/msw-handlers.ts` | `http.post(api("/stages/markdown-preview"), …)` trả golden. |
| `apps/web/src/shared/ui/markdown-content.tsx` | `export const proseClass`; bổ sung style `h4`, `del`, `hr`, `table/th/td`, `img` (border-radius theo token, `display:block`). |
| `apps/web/src/shared/ui/tabs.tsx` (mới) | `Tabs`, `TabsList`, `TabsTrigger`, `TabsContent` từ `radix-ui`, style theo token, trigger ≥ 44px. |

## Steps

1. **Dependency**
   - `pnpm --dir apps/web add @mdxeditor/editor@~4.3.2`.
   - `pnpm --dir apps/web why lexical` → đọc phiên bản resolve (ví dụ `0.48.x`); thêm
     `lexical@<x> @lexical/list@<x> @lexical/link@<x> @lexical/rich-text@<x>` với `--save-exact`.
   - Chạy lại `pnpm why lexical` → **đúng một** phiên bản; nếu ra hai, dừng và điều chỉnh (không dùng
     `overrides` trừ khi bắt buộc; nếu phải dùng, ghi lý do trong PR).
   - Kiểm `pnpm --dir apps/web build` vẫn xanh (MDXEditor chỉ ESM, Vite 8 xử lý được; chưa import ở đâu nên bundle
     không đổi).
2. **Upload dùng chung**
   - Chuyển `putToStorage` + `UploadFailed` sang `api/upload-media.ts` (giữ nguyên comment về header/credentials).
   - `export async function uploadMedia(kind: UploadRequest["kind"], file: File, opts: {onProgress?: (pct: number) => void; signal?: AbortSignal}): Promise<Media>`:
     `initUpload({kind, fileName: file.name, contentType: file.type, sizeBytes: file.size}, signal)` →
     `putToStorage(ticket.uploadUrl, file, …)` → `completeUpload(ticket.mediaId, signal)`.
   - `useVideoUpload` gọi `uploadMedia("video", file, …)`; test hiện có của dialog video (trong
     `stage-detail-page.test.tsx`) phải xanh không sửa.
3. **`model/image-file.ts`** (thuần, không import React)
   - `IMAGE_CONTENT_TYPES = ["image/png", "image/jpeg", "image/webp", "image/gif"] as const` (khớp `allowedTypes`).
   - `imageFileError(file)`: sai type → "Chỉ hỗ trợ ảnh PNG, JPEG, WebP hoặc GIF."; > 10 MB →
     `File vượt dung lượng cho phép (tối đa 10 MB).`; ngược lại `null`.
   - `altFromFileName("so-do_bai 1.final.png")` → `"so-do_bai 1.final"` (chỉ bỏ đuôi cuối, trim, rỗng thì `"Ảnh"`).
   - `mediaContentUrl(id) = \`/api/v1/media/${id}/content\``.
   - Test: bảng type/size, alt nhiều dấu chấm, tên không đuôi, tên chỉ có đuôi (`.png` → `"Ảnh"`).
4. **`model/markdown-rules.ts`** (thuần)
   - **Đo đúng thứ server giới hạn**: `MaxBodyBytes` (64 KB) áp lên **body JSON**, không phải source. JSON escape
     `\n`, `"`, `\` thành 2 byte, và MDXEditor còn thêm `\` khi xuất (báo cáo nghiên cứu §5), nên không thể đặt một
     ngưỡng source cố định. <!-- Updated: Red Team Session 1 - body size -->
   - `LESSON_BODY_MAX_BYTES = 64 * 1024` (bằng `MaxBodyBytes` của `httpx/bind.go`).
   - `utf8Bytes(s) = new TextEncoder().encode(s).length`; `jsonBodyBytes(body) = utf8Bytes(JSON.stringify(body))`
     (chính là chuỗi `http()` gửi đi — kiểm lại trong `shared/api`).
   - `lessonBodySizeError(body)`: `jsonBodyBytes(body) > LESSON_BODY_MAX_BYTES` → `Nội dung quá dài (${fmtKB} / 64 KB).`;
     ngược lại `null`. Trang soạn gọi với đúng body sẽ gửi (POST hoặc PATCH), và bộ đếm "x / 64 KB" hiển thị cùng số
     này. Body preview `{markdownSource}` luôn nhỏ hơn body lưu nên không cần kiểm riêng.
   - `MEDIA_SRC = /^\/api\/v1\/media\/[0-9a-f-]{36}\/content$/` (khớp `mediaContentPath` Go);
     `SAFE_URL = /^(https?:|mailto:|\/(?!\/)|#)/i`.
   - `markdownSourceError(src)`: rỗng/chỉ khoảng trắng → `LESSON_MARKDOWN_REQUIRED` (dùng lại hằng trong
     `lesson-form.ts`); chứa `data:image/` → `MARKDOWN_EMBEDDED_IMAGE` (hằng mới, cùng chữ với `ErrEmbeddedImage` của
     API: "Ảnh trong markdown phải tải lên, không nhúng base64."); ngược lại `null`.
   - Ảnh ngoài và link không an toàn **không** kiểm bằng regex trên chuỗi markdown: phase 3 kiểm trên cây Lexical
     (`MarkdownEditorHandle.violations()`), bắt được cả nội dung cũ nạp lúc mount mà transform không chạy. Server vẫn
     là chốt cuối (422).
   - Test: tiếng Việt có dấu (đếm byte > số ký tự); body nhiều xuống dòng/ngoặc kép/`\` (source < 64 KB nhưng JSON
     > 64 KB → lỗi); đúng biên 64 KB; `MEDIA_SRC` với uuid hợp lệ/không hợp lệ/có query; `SAFE_URL` với
     `javascript:`, `//evil`, `data:`, `/a`, `#x`, `mailto:`.
5. **Client preview**: `stagesApi.previewMarkdown = (markdownSource: string, signal?: AbortSignal) => http("/stages/markdown-preview", { method: "POST", body: { markdownSource }, schema: markdownPreviewSchema, signal })`;
   thêm golden + handler MSW.
6. **Prose dùng chung**: đổi `const proseClass` → `export const proseClass`; thêm
   `[&_h4]:mt-5 [&_h4]:mb-2 [&_h4]:font-semibold`, `[&_del]:line-through`, `[&_hr]:my-6 [&_hr]:border-line`,
   `[&_table]:mb-3 [&_table]:w-full [&_table]:border-collapse [&_th]:border [&_td]:border [&_th]:border-line [&_td]:border-line [&_th]:px-3 [&_td]:px-3 [&_th]:py-2 [&_td]:py-2 [&_th]:text-left`,
   `[&_img]:block [&_img]:my-3 [&_img]:rounded-md` — token `--color-line` đã có trong `src/styles` nên `border-line` hợp lệ; không hex thô.
   Bảng rộng: bọc bằng `overflow-x-auto` không làm được qua CSS selector trên `table`, nên thêm
   `[&_table]:block [&_table]:overflow-x-auto` (giữ `max-w-[72ch]`).
7. **Tabs**: `shared/ui/tabs.tsx` theo phong cách `select.tsx` (import từ `radix-ui`, `cn`, token), trigger
   `min-h-11` (44px), focus ring như button.

## Verification

```bash
cd apps/web
pnpm why lexical                       # một phiên bản duy nhất
pnpm test -- src/features/stages       # image-file, markdown-rules, dialog video vẫn xanh
pnpm typecheck && pnpm lint && pnpm lint:design
pnpm build                             # bundle chưa đổi vì chưa import editor
```

Trang học viên (`features/learning`) render bài markdown không đổi giao diện ngoài các phần tử mới có style
(bảng/hr/h4/del) — kiểm bằng test hiện có của learning và `make render-check` ở phase 5.

## Risk

- Lexical lệch phiên bản → transform phase 3 không chạy: chặn bằng `pnpm why` và test unit transform ở phase 3.
- Đổi style prose ảnh hưởng trang học viên: chỉ thêm selector cho phần tử chưa có style, không đổi selector cũ.
- Refactor upload video có thể đổi thời điểm báo lỗi/hủy: giữ nguyên API của `useVideoUpload`, test cũ là chốt.

## Rollback

Revert commit; `pnpm install` lại theo lockfile cũ. Không có thay đổi API/DB trong phase này.
