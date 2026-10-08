# Code simplifier: trình soạn MDXEditor cho học liệu markdown

Phạm vi: code của plan `261007-1015-mdxeditor-lesson-editor`. Không đổi hành vi, hợp đồng API, tên export dùng nơi khác,
chuỗi tiếng Việt, aria label hay tên test.

## Thay đổi

| File | Thay đổi | Lý do |
|---|---|---|
| `apps/api/internal/features/stages/service.go` (~L499, nhánh `LessonMarkdown` của `lessonContent`) | Return sớm khi có `MarkdownSource` mới; kiểm base64 chỉ còn chạy trên nội dung đang có. | Trước đây nguồn mới bị kiểm base64 hai lần (`validateMarkdownSource` rồi `strings.Contains`). Kết quả giữ nguyên: nguồn mới qua `validateMarkdownSource` thì không thể chứa `data:image/`. |
| `apps/web/src/features/stages/api/upload-media.ts:9` | Thêm `uploadErrorMessage(error)`. | Biểu thức `isApiError(e) \|\| e instanceof UploadFailed ? e.message : UPLOAD_FAILED_MESSAGE` bị lặp nguyên văn ở hai nơi. |
| `apps/web/src/features/stages/hooks/use-video-upload.ts:55` | Dùng `uploadErrorMessage`; bỏ các import không còn dùng. | DRY (như trên). |
| `apps/web/src/features/stages/components/markdown-editor/upload-image-dialog.tsx:104` | Dùng `uploadErrorMessage`; bỏ các import không còn dùng. | DRY (như trên). |
| `apps/web/src/features/stages/components/markdown-editor/gfm-guard.ts:29` | Thêm helper cục bộ `unregisterAll(offs)` dùng cho `guardText` và subscription của `gfmGuard`. | Vòng lặp teardown giống hệt nhau bị lặp lại hai lần. Thứ tự đăng ký vẫn giữ nguyên. Không dùng `mergeRegister` của `@lexical/utils` vì package đó không phải dependency trực tiếp. |
| `apps/web/src/features/stages/pages/lesson-editor-page.tsx:45,51` | Đổi `stagesCrumb` thành `STAGES_CRUMB` (để khớp với `EDITOR_CRUMB`); thêm `editorHeading(lesson)`. | Biểu thức tiêu đề trang bị lặp ở `EditorLoader` (nhánh chỉ đọc) và `LessonEditor`. |
| `apps/web/src/features/stages/pages/lesson-editor-page.tsx:~200` | Thêm `const uploadingImages = pendingUploads > 0`. | `pendingUploads > 0` từng xuất hiện bốn lần (dirty, nhãn nút, disabled, aria-busy). |
| `apps/web/src/features/stages/components/lesson-form-dialog.tsx:159` | Đổi `uploadingPct !== null` thành `uploading` (biến đã có sẵn ở L124). | Dùng cờ đã đặt tên, giống mẫu của `upload-image-dialog`. |
| `apps/web/src/features/stages/test/golden.ts:19` | Chuyển import `markdown_preview.json` về đúng vị trí theo thứ tự đường dẫn. | Đồng nhất với khối import đã sắp xếp. |
| `apps/web/e2e/lesson-editor.spec.ts:18` | Đưa `editorPath(sid, vid, lessonId)` lên cấp module, thay `editPath` cục bộ và ba template URL viết tay ở E40 (2 chỗ) và E41. | Cùng một đường dẫn trang soạn bị viết lại bốn lần. Tên test và các bước kiểm không đổi. |

## Cố ý giữ nguyên

- `event.stopPropagation()` khi submit dialog ảnh; cách đặt tên `labelEditorInternals` và selector CSS-module khớp theo
  tiền tố; CSS khối mã đơn sắc; `newParagraph` click vào h1; E36 không assert `defaultPrevented`; E37 assert
  `rel="noopener noreferrer"`: đều là lựa chọn có chủ đích theo yêu cầu.
- `NON_GFM_FORMATS` (bit số kèm comment) cạnh `BLOCKED_FORMATS`: suy bit từ bảng format của lexical sẽ khéo hơn nhưng
  khó đọc hơn, và cần kiểm xem bảng đó có được export không.
- Literal `"data:image/"` dùng hai lần trong `service.go` và một lần trong `markdown-rules.ts`: quá nhỏ để đáng tách
  hằng dùng chung giữa hai ngôn ngữ.
- `lessonEditorPath` trong `model/lesson-form.ts` tự dựng `/admin/stages/...` thay vì dùng `stagePath` (nằm ở
  `hooks/`): `model` phải là logic thuần, không import từ `hooks`.
- `pasteHtml`/`pastePng` trong E2E: payload khác nhau, phần dùng chung chỉ có một dòng dispatch.
- Go `markdown.go`, `dto.go`, `handler.go`, `editor-plugins.ts`, `editor-translation.ts`, `paste-upload.ts`,
  `markdown-editor.tsx`, `shared/ui/*`, `msw-handlers.ts` và các test helper: đã gọn và đúng convention, không có trùng lặp
  đáng sửa.

## Kiểm chứng

- `apps/web`: `corepack pnpm typecheck` sạch, `corepack pnpm lint` sạch, `corepack pnpm lint:design` 0 vi phạm,
  `corepack pnpm test` 558/558 pass (59 file).
- `apps/api`: `go vet ./...` sạch; `go test ./internal/features/stages/... ./internal/seed/...` ok; `gofmt -l` sạch.
- E2E chưa chạy (controller sẽ chạy). `lesson-editor.spec.ts` chỉ đổi cách dựng đường dẫn, typecheck và lint đều pass.
