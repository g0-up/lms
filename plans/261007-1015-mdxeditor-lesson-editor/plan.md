---
title: "Trình soạn MDXEditor cho học liệu markdown"
description: "Thay Textarea markdown trong dialog học liệu bằng trang soạn riêng dùng MDXEditor 4.3 (WYSIWYG xuất GFM), upload ảnh qua media, tab Xem trước render bằng chính renderer server, và chặn ảnh ngoài ngay khi lưu."
status: completed
priority: P2
effort: "5 phase (~5 ngày công)"
issue: ""
branch: master
tags: [feature, frontend, backend, api, editor, markdown]
blockedBy: []
blocks: []
created: 2026-10-07
---

# Trình soạn MDXEditor cho học liệu markdown

## 1. Kết quả mong muốn

Admin soạn học liệu markdown trên một **trang soạn riêng** bằng MDXEditor (WYSIWYG) thay vì gõ markdown thô vào
Textarea trong dialog, và nhận lại đúng HTML mà học viên sẽ thấy:

- Thanh công cụ chỉ có những định dạng mà pipeline server (goldmark GFM + bluemonday allowlist) giữ được: tiêu đề
  H1–H4, đậm, nghiêng, gạch ngang, danh sách chấm/số, trích dẫn, khối mã, bảng, đường kẻ, liên kết, ảnh đã upload.
  Định dạng ngoài GFM (gạch chân, sup/sub, highlight, checklist, H5/H6, style dán từ Word) bị chặn trong editor.
- Ảnh luôn đi qua luồng media hiện có (`kind: "image"`, presigned PUT), chèn dạng `/api/v1/media/{uuid}/content`.
  Dialog chèn ảnh bắt buộc alt; ảnh dán/kéo thả lấy tên file (bỏ đuôi) làm alt.
- Tab **Xem trước** gọi endpoint render của server, hiển thị bằng `MarkdownContent` như trang học viên.
- Server từ chối lưu (422) markdown có ảnh không phải ảnh media nội bộ; phát hành vẫn lọc bằng bluemonday như cũ.
- Học liệu cũ (seed, bản nháp đã nhân bản) mở ra không bị đổi source nếu người dùng không sửa gì.

## 2. Phạm vi (Scope Challenge: HOLD SCOPE)

Phạm vi người dùng giao: **api, web, database nếu cần**. Không cắt, không mở rộng.

- **Database: không cần thay đổi.** `lessons.markdown_source` là `text`, CHECK theo `type` đã đúng; không có cột,
  index hay migration mới. Không cần backup DB vì không có thay đổi schema/dữ liệu.
- **API:** luật chặn ảnh ngoài khi lưu (AddLesson/UpdateLesson) + endpoint `POST /api/v1/stages/markdown-preview`.
- **Web:** dependency MDXEditor + Lexical, wrapper editor, plugin chặn định dạng, dialog upload ảnh, trang soạn,
  tab xem trước, điều hướng từ dialog cũ, chặn rời trang khi chưa lưu, test unit/E2E/a11y, cập nhật docs.
- **Infra (tối thiểu, bắt buộc để ảnh hiển thị):** thêm `MEDIA_ORIGIN` vào CSP `img-src` của
  `infra/caddy/Caddyfile` và `infra/docker-compose.homelab.yml`. Nằm ngoài "api, web, db" nhưng không có nó ảnh markdown
  bị chặn trên production (phase 5).

### Quyết định đã chốt (Validation Session 1, 2026-10-07)

| # | Câu hỏi | Quyết định |
|---|---|---|
| D1 | Editor đặt ở đâu? | **Trang soạn riêng** (`…/versions/:versionId/lessons/new` và `…/lessons/:lessonId/edit`), không nhét vào dialog 560px. |
| D2 | Server kiểm ảnh thế nào? | **Chặn ảnh ngoài khi lưu**: AddLesson/UpdateLesson trả 422 nếu có ảnh markdown không khớp `^/api/v1/media/{uuid}/content$`. Link nguy hiểm vẫn để bluemonday lọc lúc render. |
| D3 | Alt ảnh? | **Bắt buộc trong dialog chèn ảnh**; ảnh dán/kéo thả lấy tên file bỏ đuôi làm alt. |
| D4 | Có xem trước không? | **Có, tab "Xem trước"** render bằng server (cùng renderer với lúc phát hành). |

### Ràng buộc

- MDXEditor `~4.3.2` (MIT, Lexical `^0.48`, chỉ ESM, cần `@mdxeditor/editor/style.css`). Ghim `lexical` và
  `@lexical/{list,link,rich-text}` **đúng phiên bản** MDXEditor resolve (pnpm strict) để plugin chặn định dạng import
  được node class; `pnpm why lexical` phải ra đúng một phiên bản.
- Bundle editor (~160–530 KB gzip) chỉ nằm trong chunk lazy của trang soạn; không lọt vào chunk học viên hay
  `react-vendor`.
- Kiến trúc web: `shared → features → app`; `features/<f>/model` thuần logic. Phase web chỉ sửa `src/features/stages/**`
  ngoại trừ các thay đổi foundation đã liệt kê rõ ở phase 2 (`shared/ui/markdown-content.tsx`, `shared/ui/tabs.tsx`).
- `pnpm lint:design`: không hex thô ngoài file token, không emoji, chữ ≥ 12px, vùng bấm ≥ 44px (ngoại lệ ghi
  `design-audit-allow <rule>: <reason>`).
- Giới hạn body API `MaxBodyBytes = 64 << 10` áp lên body JSON; client đo **chính body JSON sẽ gửi** bằng
  `TextEncoder` (tiếng Việt 2–3 byte/ký tự, JSON escape xuống dòng/ngoặc kép) và chặn khi > 64 KB.
- Không đưa ID plan, số phase, mã finding vào code, tên test, commit.

### Ngoài phạm vi (non-goals)

- Đổi phân quyền: soạn vẫn là **admin** (authz hiện tại admin-only cho stages/media; "giảng viên" trong yêu cầu gốc
  ánh xạ vào vai trò admin hiện có).
- Dọn ảnh mồ côi (upload rồi bỏ) — giống video hiện nay.
- Tự lưu nháp, cộng tác thời gian thực, lịch sử phiên bản trong editor, resize ảnh, MDX/JSX, footnote, HTML thô.
- Đổi pipeline render phía server (goldmark + bluemonday giữ nguyên).
- Sửa dữ liệu cũ có ảnh ngoài (không có trong seed; bản nháp cũ chứa ảnh ngoài sẽ phải gỡ ảnh khi lưu lại — xem §6).

## 3. Tiêu chí chấp nhận

1. `POST/PATCH /stage-versions/{vid}/lessons` với markdown chứa `![a](https://x/y.png)` hoặc `![a](/khac.png)` trả
   **422** (lỗi `domain.ErrInvalid`) với thông điệp "Ảnh trong markdown phải tải lên qua hệ thống."; ảnh
   `/api/v1/media/{uuid}/content` vẫn 201/200; ảnh base64 giữ thông điệp cũ.
2. PATCH chỉ đổi `title`/`required` của bài cũ **không** bị chặn bởi luật ảnh ngoài (luật chỉ chạy khi có
   `markdownSource` mới).
3. `POST /api/v1/stages/markdown-preview {markdownSource}` trả `200 {html}` bằng `NewMarkdownRenderer()`; chưa đăng
   nhập 401, không phải admin 403, body quá 64 KB trả 400 `details.body = "Dữ liệu quá lớn (tối đa 64KB)"` theo
   `BindJSON` hiện có, ảnh ngoài 422 như tiêu chí 1.
4. "Thêm học liệu" → chọn Markdown → "Tiếp tục soạn" mở trang soạn với tiêu đề/bắt buộc đã nhập; "Sửa" trên bài
   markdown mở thẳng trang soạn; bài video giữ nguyên dialog cũ.
5. Nội dung **tạo trong editor**: Ctrl+U, dán HTML có `<u>/<sup>/<mark>/style` không tạo định dạng ngoài GFM; H5/H6
   thành H4; checklist thành danh sách chấm; link `javascript:` bị gỡ; ảnh không phải media bị gỡ. Nội dung **cũ**
   (transform không chạy lúc mount) có ảnh/link vi phạm thì `violations()` liệt kê và chặn lưu khi đã sửa nội dung.
6. Chèn ảnh qua dialog bắt buộc alt; dán/kéo thả ảnh upload qua media và có alt = tên file bỏ đuôi; upload lỗi hiện
   toast, không để lại ảnh hỏng; tối đa 5 ảnh đang tải, nút Lưu khóa khi còn ảnh đang tải.
7. Mở **mọi** bài markdown seed (7 bài) rồi "Lưu" không sửa nội dung → `markdown_source` trong DB **không đổi byte
   nào**, và HTML phát hành khớp bản trước. Refetch dữ liệu trong lúc soạn không làm PATCH ghi đè source.
8. Tab "Xem trước" hiển thị HTML server giống trang học viên (cùng `MarkdownContent` và class prose).
9. Rời trang khi có thay đổi chưa lưu → hỏi xác nhận (điều hướng trong app và `beforeunload`); không hỏi sau khi lưu
   thành công và không chặn chuyển về `/login` khi phiên hết hạn (401).
10. Phiên bản không phải draft hoặc bài không tồn tại → trang soạn hiện trạng thái lỗi/chỉ đọc, không cho lưu.
11. `make check` xanh; `make test-api` với `TEST_DATABASE_URL` xanh; E2E mới + E2E cũ (E02, E06, E22) xanh; ảnh trong
    editor/xem trước có `naturalWidth > 0`; axe không có vi phạm trên trang soạn; chunk editor không có trong bundle
    của khu học viên; CSP `img-src` gồm `MEDIA_ORIGIN`.
12. `docs/architecture.md` cập nhật endpoint preview, luật ảnh khi lưu, trang soạn và câu CSP (`img-src`).

## 4. Phases

| # | Phase | Phụ thuộc | Ước lượng | Trạng thái |
|---|---|---|---|---|
| 1 | [API: chặn ảnh ngoài khi lưu + endpoint xem trước](phase-01-api-image-rule-preview.md) | — | 0.75 ngày | completed |
| 2 | [Web foundation: dependency, upload media dùng chung, quy tắc markdown, client preview, prose](phase-02-web-foundation.md) | 1 (hợp đồng API) | 1 ngày | completed |
| 3 | [Component MarkdownEditor: plugin, chặn định dạng, dialog ảnh, tiếng Việt, theme](phase-03-markdown-editor-component.md) | 2 | 1.5 ngày | completed |
| 4 | [Trang soạn + luồng điều hướng + chặn rời trang](phase-04-lesson-editor-page-flow.md) | 2, 3 | 1 ngày | completed |
| 5 | [E2E, a11y, bundle, CSP, docs](phase-05-e2e-a11y-docs.md) | 1–4 | 0.75 ngày | completed |

Phase 1 và 2 chạy song song được (sở hữu file tách biệt: `apps/api/**` vs `apps/web/**`); 3 → 4 → 5 tuần tự.

## 5. Kiến trúc tóm tắt

```text
Trang soạn (lazy chunk)                                    API (Go)
┌──────────────────────────────────────┐   PATCH/POST lessons   ┌───────────────────────────────┐
│ Tiêu đề · Bắt buộc                   │ ─────────────────────▶ │ contentFactory                │
│ [Soạn thảo] [Xem trước]              │                        │  ├ data:image → 422 (cũ)      │
│  MarkdownEditor (MDXEditor)          │                        │  └ ast.Image src ≠ media → 422│
│   ├ plugins GFM-only                 │   POST markdown-preview│ PreviewMarkdown               │
│   ├ gfmGuard (transform + command)   │ ─────────────────────▶ │  └ cùng luật → Render → {html}│
│   ├ hardBreak export                 │                        │ Publish: Render + bluemonday  │
│   └ UploadImageDialog ─ uploadMedia ─┼─▶ /media/uploads → PUT storage → complete             │
│ Submit: getMarkdown() nếu đã sửa;     │                        └───────────────────────────────┘
│  body JSON ≤ 64KB, violations() rỗng, │
│  không còn ảnh đang tải              │
└──────────────────────────────────────┘
```

Ba lớp bảo vệ độ trung thực: (1) editor chỉ cho tạo cấu trúc GFM, (2) kiểm khi submit ở client (body JSON,
`violations()` ảnh/link, upload đang chạy), (3) server chặn ảnh ngoài khi lưu và vẫn sanitize khi render.

## 6. Rủi ro chính

| Rủi ro | Mức | Giảm thiểu |
|---|---|---|
| MDXEditor normalize source khi mount (`_em_`→`*em*`, `-`→`*`, bảng đệm…) làm "bẩn" form và đổi source cũ | Cao | Bỏ qua lần `onChange(md, initialMarkdownNormalize=true)`; chỉ gửi `markdownSource` khi người dùng sửa thật; test round-trip seed (tiêu chí 7). |
| Source có HTML thô/footnote/reference link làm editor `onError` và trắng nội dung | Trung bình | `suppressHtmlProcessing`; `onError` hiện Alert và chuyển sang chế độ "Mã nguồn" (`diffSourcePlugin`) để sửa; seed đã kiểm không có các cú pháp này ngoài code block. |
| `ImageNode.setAltText` không public (chưa xác minh do dist bị chặn) | Trung bình | Spike đầu phase 3; fallback: handler `PASTE_COMMAND`/`DROP_COMMAND` riêng tự upload rồi `insertImage$` với alt. |
| Lệch phiên bản Lexical giữa MDXEditor và import trực tiếp → node class khác nhau, transform không chạy | Trung bình | Ghim đúng phiên bản, `pnpm why lexical` một phiên bản, test unit transform. |
| Bản nháp cũ chứa ảnh ngoài không lưu được nội dung mới | Thấp | Thông điệp 422 rõ; đổi tiêu đề/bắt buộc vẫn được (luật chỉ chạy khi có source mới); seed không có ảnh ngoài. |
| Bundle phình | Trung bình | Lazy route + kiểm chunk trong phase 5. |
| jsdom thiếu API layout cho Lexical | Thấp | Polyfill `Range.getBoundingClientRect/getClientRects`; hành vi gõ/dán/dialog kiểm bằng Playwright. |
| CSP `img-src 'self' blob:` chặn redirect `/content` → R2 trên production | Cao | Thêm `MEDIA_ORIGIN` vào `img-src` (phase 5); E2E kiểm `naturalWidth`; kiểm tay trên staging vì stack E2E không dùng CSP production. |
| Hai admin cùng sửa **nội dung** một bài → last-write-wins | Thấp | Giữ như hiện nay (API không có token phiên bản; ngoài phạm vi). Sửa tiêu đề không ghi đè source nhờ `contentTouched`. |
| Khối mã ngôn ngữ lạ (`python`, `tsx`…) mất nội dung/ngôn ngữ | Trung bình | Descriptor dự phòng giữ `language`/`meta`; spike S2 + test phase 3. |

## 7. Tài liệu tham khảo

- Nghiên cứu API MDXEditor v4: [`plans/reports/researcher-261007-1015-mdxeditor-v4-api.md`](../reports/researcher-261007-1015-mdxeditor-v4-api.md) (§11 cấu hình khuyến nghị).
- Plan MVP gốc (luồng học liệu, media): [`plans/261005-0743-lms-mvp-fullstack/plan.md`](../261005-0743-lms-mvp-fullstack/plan.md).
- Mã hiện có: `apps/api/internal/features/stages/{markdown.go,service.go,handler.go}`,
  `apps/web/src/features/stages/**`, `apps/web/src/shared/ui/markdown-content.tsx`.

## Validation Log

### Session 1 — 2026-10-07

- D1 Editor placement → Trang soạn riêng.
- D2 Server image check → Chặn ảnh ngoài khi lưu (Recommended).
- D3 Image alt → Bắt buộc trong dialog, paste/drop lấy tên file (Recommended).
- D4 Preview → Có, thêm tab Xem trước.

## Red Team Review

### Session 1 — 2026-10-07

Ba reviewer (giả định, lỗi vận hành, bảo mật/phạm vi). Mỗi finding được đối chiếu với mã (file:dòng) trước khi quyết;
gộp trùng còn 15 mục.

| # | Finding | Severity | Disposition | Applied To |
|---|---|---|---|---|
| 1 | Submit so `values.markdownSource` với dữ liệu query đang sống; refetch lúc focus làm PATCH ghi đè source người khác, và chế độ Mã nguồn có thể không cập nhật field | Critical | Accept: `contentTouched` + `getMarkdown()` là nguồn duy nhất; default nạp một lần; spike S1 kiểm `onChange` ở chế độ Mã nguồn | Phase 3, 4; tiêu chí 7 |
| 2 | CSP production `img-src 'self' blob:` chặn redirect `/content` → R2, ảnh markdown không hiện | Critical | Accept: thêm `MEDIA_ORIGIN` vào `img-src` (Caddy + homelab), sửa docs, E2E kiểm `naturalWidth` | Phase 5; §2, tiêu chí 11–12 |
| 3 | Luật ảnh ngoài làm vỡ `TestPublishOrderAndAudit` và `TestPublishMarkdownOnRealSchema`; mã lỗi bind ghi sai (`VALIDATION`) | High | Accept: seed nội dung cũ qua fake repo/testdb, giữ assert strip khi phát hành; sửa thành `VALIDATION_FAILED`; integration test bắt buộc | Phase 1 |
| 4 | Giới hạn 60 KB đo trên source, trong khi server giới hạn 64 KB trên body JSON (escape xuống dòng/ngoặc kép) | High | Accept: `jsonBodyBytes`/`lessonBodySizeError` đo body thật | Phase 2, 4; §2 ràng buộc |
| 5 | Transform không chạy lúc mount: bài cũ có ảnh/link vi phạm lọt qua client | High | Accept: `MarkdownEditorHandle.violations()` chặn submit và liệt kê | Phase 3, 4; tiêu chí 5 |
| 6 | Blocker giữ người dùng khi 401 chuyển về `/login`, giữ nội dung phiên cũ trên màn hình | High | Accept: bỏ qua `LOGIN_PATH`/`FIRST_LOGIN_PATH`; test 401 không hiện dialog | Phase 4; tiêu chí 9 |
| 7 | Lưu khi ảnh dán còn đang tải làm mất ảnh hoặc chèn ảnh hỏng | High | Accept: `onPendingUploadsChange`, khóa Lưu, hàng đợi tối đa 5, E2E giữ PUT bằng `page.route` | Phase 3, 4, 5; tiêu chí 6 |
| 8 | Khối mã ngôn ngữ không khai báo (`python`) và `jsx` của seed bị mất/đổi; golden chỉ có 5/7 bài seed | High | Accept: descriptor dự phòng, thêm `tsx`/`jsx`; golden `seed_markdown.json` đủ 7 bài dùng chung API/web; E40 phủ mọi chặng seed | Phase 1, 2, 3, 5 |
| 9 | `form.reset()` trước `navigate` không tắt kịp `useBlocker` (effect chạy sau render) | Medium | Accept: chỉ dùng ref `allowLeave`; test lưu xong không hiện dialog | Phase 4 |
| 10 | Query preview dưới key `["stages", …]` bị invalidate sau mỗi lần lưu; `meta.silent` không có tác dụng | Medium | Accept: root key riêng `["stage-markdown-preview", source]`, bỏ `meta` | Phase 4 |
| 11 | `VERSION_IMMUTABLE` khi lưu làm mất công soạn; test "explains VERSION_IMMUTABLE inside the dialog" vỡ khi bài markdown rời dialog | Medium | Accept: editor readOnly + "Sao chép markdown"; chuyển test, thêm fixture bài video | Phase 4 |
| 12 | `COMMAND_PRIORITY_CRITICAL + 1` không hợp lệ (0–4); quy tắc gỡ `ImageNode` có ngoại lệ blob mơ hồ | Medium | Accept: Cách B (handler CRITICAL hoặc realmPlugin đăng ký trước `imagePlugin`); một quy tắc gỡ `!MEDIA_SRC` | Phase 3 |
| 13 | E2E yếu: Ctrl+U không có đối chứng dương, `await page.reload()` treo với `beforeunload`, drop không chứng minh đã xử lý, a11y spec cần `seedReset`, paste trên webkit | Medium | Accept: đối chứng `ControlOrMeta+B`, reload không await, `defaultPrevented`, E42 vào spec `@serial`, `afterAll(seedReset)`, skip webkit có lý do | Phase 5 |
| 14 | Thiếu token phiên bản: hai admin sửa nội dung cùng lúc → last-write-wins | Medium | Reject: hành vi hiện có của toàn bộ lesson API; non-goal; ghi rủi ro (§6) | §6 |
| 15 | Đề xuất rate limit preview, trả `details` trong 422, hoặc đổi preview sang hợp đồng cảnh báo | Low | Reject: endpoint admin-only, body ≤ 64 KB, rate limit cần sửa `router.go`; `domain.Error` không có Details và `violations()` đã liệt kê phía client; giữ 422 đồng nhất với lúc lưu | — |

### Whole-Plan Consistency Sweep

- Rà các thuật ngữ cũ (`60 KB`, `MARKDOWN_MAX_BYTES`, so sánh `keepSource` với query, `form.reset`,
  `["stages", "markdown-preview"`, `5 bài seed`, `TestHandlerUnauthenticated`, `COMMAND_PRIORITY_CRITICAL + 1`) trên
  mọi phase; sở hữu file không chồng chéo giữa các phase; link và frontmatter hợp lệ.
