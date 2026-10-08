---
phase: 1
title: "Phase 1: API: chặn ảnh ngoài khi lưu + endpoint xem trước"
status: completed
priority: P2
effort: "0.75 ngày"
dependencies: []
---

# Phase 1: API: chặn ảnh ngoài khi lưu + endpoint xem trước

## Goal

Server từ chối lưu markdown có ảnh không phải ảnh media nội bộ (quyết định D2) và cung cấp
`POST /api/v1/stages/markdown-preview` để web hiển thị đúng HTML mà học viên sẽ thấy (D4). Không đổi schema DB.

## Context & Requirements

- **Sở hữu file:** `apps/api/internal/features/stages/**`, `apps/api/internal/seed/seed_test.go` +
  `apps/api/internal/seed/testdata/seed_markdown.json` (golden nguồn seed cho web), và một test trong
  `apps/api/internal/app/router_test.go`. Không sửa `internal/app/router.go` (stages đã
  `Register(authed, requireAdmin)`, route mới nằm trong group `/stages` nên tự có session + admin).
- Hiện trạng (`service.go:465` `contentFactory`): nhánh `LessonMarkdown` chỉ kiểm `strings.Contains(c.Source,
  "data:image/")` → `ErrEmbeddedImage` (422, `domain.ErrInvalid`). Kiểm đó chạy cả khi PATCH không gửi
  `markdownSource` (dùng `current`) — giữ nguyên hành vi này.
- `markdown.go` đã có `mediaContentPath`, `mediaSrcPattern = ^/api/v1/media/([0-9a-f-]{36})/content$`,
  `GoldmarkRenderer` (goldmark GFM, không `unsafe`, bluemonday `NewPolicy` allowlist).
- `domain.ErrInvalid` → 422; `httpx.BindJSON` → 400 `VALIDATION_FAILED` (unknown field, body > 64 KB với
  `details.body = "Dữ liệu quá lớn (tối đa 64KB)"`).
- E2E `E22` (`apps/web/e2e/learning.spec.ts:313`) gửi `<img src="x" onerror=…>` dạng **HTML thô**: đó là
  `ast.HTMLBlock`, không phải `ast.Image`, nên luật mới không chặn và E22 phải vẫn 201 — giữ nguyên ý nghĩa test lọc
  XSS.
- **Hai test hiện có sẽ đỏ** vì thêm ảnh ngoài qua `AddLesson` để chứng minh phát hành lọc ảnh ngoài:
  `TestPublishOrderAndAudit` (`service_test.go:421-422`) và `TestPublishMarkdownOnRealSchema`
  (`repository_pg_test.go:287`, qua `e.addMarkdown` → `AddLesson`). **Không** xóa ảnh ngoài khỏi hai test này: chúng
  là bằng chứng duy nhất cho "phát hành vẫn lọc". Đưa source cũ vào thẳng repo giả / `testdb` (như ca "dữ liệu cũ"
  của UpdateLesson) để luật mới không chặn. <!-- Updated: Red Team Session 1 - existing publish tests -->
- **Clone không đi qua `contentFactory`** (`CloneAsDraft` → `copyContent`, `entity.go:349-362`): bài cũ có ảnh ngoài
  vẫn nhân bản được, chỉ bị chặn khi lưu source mới.

## Thiết kế

1. **Phát hiện ảnh ngoài bằng AST, không regex**: parse source bằng parser goldmark dựng từ **cùng biến danh sách
   extension** với `NewMarkdownRenderer` (`markdownExtensions`), để luật kiểm và renderer không lệch nhau khi sau này
   thêm extension;
   `ast.Walk` lấy mọi `*ast.Image`, so `string(img.Destination)` với `mediaSrcPattern`. Bao cả ảnh inline
   `![a](url)`, ảnh reference `![a][r]` (parser đã resolve destination), ảnh trong bảng/trích dẫn/danh sách. Code
   block và inline code không sinh `ast.Image` nên không bị bắt nhầm.
2. **Luật chỉ chạy khi có source mới** (`cmd.MarkdownSource != nil`): đổi tiêu đề/bắt buộc của bài cũ không bị chặn.
3. Thứ tự kiểm: `data:image/` (giữ thông điệp cũ, giữ chạy cả với `current`) → ảnh ngoài (chỉ source mới).
4. **Preview** là hàm thuần trên service, không đọc DB, không transaction, cùng luật kiểm ảnh rồi `s.render.Render`.

## Files

| File | Thay đổi |
|---|---|
| `apps/api/internal/features/stages/markdown.go` | Tách `markdownExtensions = []goldmark.Extender{extension.GFM}` dùng cho cả `NewMarkdownRenderer` và biến package `markdownParser`; thêm `foreignImages(source string) []string`; import `github.com/yuin/goldmark/ast`, `github.com/yuin/goldmark/text`. Không đổi interface `MarkdownRenderer` (có fake `upperRenderer` trong test). |
| `apps/api/internal/features/stages/service.go` | `ErrExternalImage = domain.ErrInvalid.WithMsg("Ảnh trong markdown phải tải lên qua hệ thống.")`; hàm `validateMarkdownSource(src string) error` (data:image → `ErrEmbeddedImage`, ảnh ngoài → `ErrExternalImage`); gọi trong `contentFactory` khi `cmd.MarkdownSource != nil`; giữ kiểm `data:image/` hiện có cho `current`; thêm `func (s *Service) PreviewMarkdown(source string) (string, error)`. |
| `apps/api/internal/features/stages/dto.go` | `previewRequest{MarkdownSource string \`json:"markdownSource"\`}`, `previewResponse{HTML string \`json:"html"\`}`. |
| `apps/api/internal/features/stages/handler.go` | `st.POST("/markdown-preview", h.previewMarkdown)`; handler `BindJSON` → `PreviewMarkdown` → `httpx.OK(c, 200, previewResponse{…})`, lỗi qua `fail(c, err)` như các handler khác. |
| `apps/api/internal/features/stages/markdown_test.go` | Test bảng cho `foreignImages`. |
| `apps/api/internal/features/stages/service_test.go` | Thêm case vào bảng validation `AddLesson` (~dòng 315) và test UpdateLesson/Preview; mở rộng `TestHandlerStatusAndEnvelopes` cho route mới (200/422/400). Sửa `TestPublishOrderAndAudit` (~dòng 421) để đưa source có ảnh ngoài vào repo giả thay vì `AddLesson`, giữ nguyên assert phát hành lọc ảnh ngoài. |
| `apps/api/internal/features/stages/repository_pg_test.go` | Sửa `TestPublishMarkdownOnRealSchema` (~dòng 287) tương tự: chèn bài có ảnh ngoài qua SQL/`testdb`, giữ assert. |
| `apps/api/internal/app/router_test.go` | Thêm `TestStagesPreviewRequiresSession` theo mẫu `TestLearningRoutesRequireSession` (~dòng 157): `POST /api/v1/stages/markdown-preview` không phiên → 401 `UNAUTHENTICATED`. Đây là test duy nhất chứng minh route nằm sau guard (handler test dựng route không guard). |
| `apps/api/internal/features/stages/repository_pg_test.go` (golden) | Thêm một lời gọi `e.call(r, POST, "/api/v1/stages/markdown-preview", …, 200)` vào `TestGoldenJSON` (~dòng 889) để ghi golden `markdown_preview.json`. |
| `apps/api/internal/seed/seed_test.go` + `testdata/seed_markdown.json` | `TestSeedMarkdownGolden`: ghi `[{key, source}]` của **mọi** bài markdown seed (7 bài, từ `data.go`) ra golden với cờ `-update` theo quy ước golden hiện có; không cờ thì so khớp. Web (phase 2–3) dùng file này để test round-trip editor trên nội dung seed thật, gồm cả fence `jsx`. Không cần DB. |
| `apps/api/internal/features/stages/testdata/markdown_preview.json` | Golden response cho MSW (phase 2), sinh bằng `go test -tags integration -run TestGoldenJSON -update`. |

## Steps

1. `markdown.go`
   - `markdownExtensions` dùng chung; `markdownParser = goldmark.New(goldmark.WithExtensions(markdownExtensions...)).Parser()`
     ở package level. Parser của goldmark tạo context mới mỗi lần `Parse`, nên dùng song song an toàn (kiểm lại doc
     goldmark khi cài; nếu không chắc, tạo parser mỗi lần gọi).
   - `func foreignImages(source string) []string`: `doc := markdownParser.Parse(text.NewReader([]byte(source)))`,
     `ast.Walk(doc, func(n ast.Node, entering bool) …)`, với `*ast.Image` khi `entering` và
     `!mediaSrcPattern.MatchString(string(img.Destination))` thì append destination. Trả slice (dùng cho log/test).
2. `service.go`
   - Khai báo `ErrExternalImage` cạnh `ErrEmbeddedImage` (comment tiếng Việt cùng phong cách).
   - `func validateMarkdownSource(src string) error`.
   - Trong `contentFactory` nhánh markdown:
     ```go
     if cmd.MarkdownSource != nil {
         if err := validateMarkdownSource(*cmd.MarkdownSource); err != nil {
             return nil, err
         }
         c = MarkdownContent{Source: *cmd.MarkdownSource}
     }
     if strings.Contains(c.Source, "data:image/") { // giữ: bài cũ lỡ có base64 vẫn bị chặn như trước
         return nil, ErrEmbeddedImage
     }
     ```
   - `PreviewMarkdown(source string) (string, error)`: `validateMarkdownSource` rồi `s.render.Render(source)`.
3. `dto.go` + `handler.go`: request/response và route như bảng Files. Route khai báo **trước** `st.GET("/:id")`
   không cần thiết (khác method) nhưng đặt ngay sau `st.POST("", h.create)` cho dễ đọc.
4. Tests (tên test mô tả hành vi, không chứa số phase):
   - `TestForeignImages`: `![a](https://x/y.png)` → 1; `![a](/api/v1/media/<uuid>/content)` → 0;
     `![a](/api/v1/media/<uuid>/content?x=1)` → 1; `![a][r]\n\n[r]: https://x/y.png` → 1;
     `` `![a](https://x)` `` và fenced code chứa ảnh → 0; `<img src="https://x">` (HTML thô) → 0; ảnh trong bảng → 1.
   - Bảng validation `AddLesson`: thêm `{"ảnh ngoài", markdownCmd("A", "![x](https://cdn.example/a.png)"), ErrExternalImage}`,
     `{"ảnh đường dẫn khác", markdownCmd("A", "![x](/uploads/a.png)"), ErrExternalImage}`; case hợp lệ với ảnh media.
   - `UpdateLesson`: bài có source chứa ảnh ngoài được chèn trực tiếp qua repo/`testdb` (giả lập dữ liệu cũ) → PATCH
     `{title}` thành công; PATCH `{markdownSource: "<ảnh ngoài>"}` → `ErrExternalImage`.
   - `PreviewMarkdown`: trả HTML có `<h1>`, bỏ `<script>`; ảnh ngoài → `ErrExternalImage`; chuỗi rỗng → `""`.
   - Handler: `POST /api/v1/stages/markdown-preview` → 200 `{html}`; ảnh ngoài → 422 với message; field lạ → 400;
     handler này không gọi `mustActor` nên **không** thêm vào `TestHandlerUnauthenticated` (route trong handler test
     không có guard). 401 kiểm ở `router_test.go` (`TestStagesPreviewRequiresSession`); 403 cho giảng viên kiểm ở E2E
     phase 5 (E38).
   - Golden `testdata/markdown_preview.json`: thêm vào `TestGoldenJSON` (`repository_pg_test.go`, router dựng bằng
     `e.router()` với admin cố định), body là source markdown của bài seed đầu tiên (`# …`, danh sách, code) để
     fixture có HTML thật; chạy `go test -tags integration ./internal/features/stages/ -run TestGoldenJSON -update` (cần Postgres của
     `make dev`) rồi commit file.

## Verification

```bash
cd apps/api && go test ./internal/seed -run TestSeedMarkdownGolden -update   # sinh golden seed (không cần DB; seed_test.go khai báo cờ update riêng vì các test khác của package có tag integration)
cd apps/api && go test ./internal/features/stages/... ./internal/app/... ./internal/seed/... -count=1   # toàn package, không -run: bắt cả test publish cũ
TEST_DATABASE_URL=… make test-api   # bắt buộc chạy integration (TestPublishMarkdownOnRealSchema, golden) trước khi đóng phase
make lint                # golangci-lint + web lint/typecheck không bị ảnh hưởng
```

E2E hiện có phải xanh ở phase 5: `E06` (`# E06`), `E22` (HTML thô + ảnh media).

## Risk

- Destination ảnh có ký tự escape/entity: goldmark trả **bytes thô** (unescape chỉ xảy ra lúc render,
  `renderer/html` → `util.URLEscape`). Regex media chỉ khớp chuỗi chuẩn, nên biến thể escape bị chặn (chặt hơn
  render, không phải lỗ hổng) — đúng ý D2. <!-- Updated: Red Team Session 1 - fact correction -->
- Ảnh reference không có definition goldmark render thành text, không sinh `ast.Image` → không chặn (đúng, vì không
  render ra ảnh).
- Không ảnh hưởng `checkMarkdownImages` lúc publish (vẫn kiểm media ready + kind image).
- **Không thêm rate limit cho preview** (đã cân nhắc): route chỉ admin, có CSRF header, body ≤ 64 KB, cùng chi phí với
  PATCH bài hiện không có limiter; web chỉ gọi khi mở tab Xem trước. Thêm limiter cần sửa `router.go` — ngoài phạm vi.
- **Preview fail-closed với bài cũ có ảnh ngoài** (422): chấp nhận; trang soạn đã liệt kê ảnh vi phạm trước khi gọi
  (phase 4, kiểm khi submit) nên admin biết cần gỡ ảnh nào.

## Rollback

Revert các file trong `apps/api/internal/features/stages/**`. Không có migration, không có dữ liệu cần khôi phục.
