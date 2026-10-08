---
phase: 5
title: "Phase 5: E2E, a11y, bundle, docs"
status: completed
priority: P2
effort: "0.75 ngày"
dependencies: [1, 2, 3, 4]
---

# Phase 5: E2E, a11y, bundle, docs

## Goal

Kiểm chứng trên trình duyệt thật (stack compose profile `full`) những hành vi jsdom không kiểm được:

- gõ, dán và kéo thả;
- Ctrl+U;
- upload ảnh qua storage thật;
- round-trip nội dung seed;
- chặn rời trang;
- a11y bằng axe.

Ngoài ra, xác nhận editor không lọt vào bundle của học viên, sửa CSP `img-src` để ảnh markdown hiển thị được trên
production, và cập nhật docs.

## Context & Requirements

- **Sở hữu file:**
  - `apps/web/e2e/lesson-editor.spec.ts` (mới);
  - `apps/web/e2e/playwright.config.ts` (thêm spec vào `E2E_SPECS`);
  - `apps/web/e2e/support/*` nếu cần helper;
  - `infra/caddy/Caddyfile`, `infra/docker-compose.homelab.yml` (chỉ chuỗi CSP `img-src` và comment);
  - `docs/architecture.md`.
  - **Không** sửa `apps/web/e2e/a11y.spec.ts`: test axe của trang soạn nằm trong `lesson-editor.spec.ts` (cần bản nháp
    và `seedReset`). <!-- Updated: Red Team Session 1 - a11y placement -->
- **Hạ tầng E2E:** `make e2e` dựng stack (postgres, minio, mailpit, api, worker, web :8081) và chạy project `e2e`
  cùng `a11y`. Các spec dùng chung một DB seed, một worker, chạy tuần tự. Spec nào làm đổi seed thì theo mẫu
  `content-versioning.spec.ts`: `test.describe(…, { tag: "@serial" })`, `describe.configure({ mode: "serial" })`,
  và `seedReset()` ở `beforeAll`/`afterAll`.
- **Ảnh mẫu:** dùng `PIXEL_PNG` có sẵn trong `learning.spec.ts`. Chuyển nó sang `e2e/support` nếu cần dùng chung.
  Không thêm file nhị phân lớn.
- **`checkFlowRoutes()`** (`e2e/support/a11y.ts`) so khớp với prototype cho render-check. Trang soạn **không** có
  trong prototype, nên **không** thêm vào danh sách đó. Viết test a11y riêng.
- Mã test E đánh tiếp sau E32 hiện có (E33…), theo quy ước đặt tên test E2E của repo.
- **CSP chặn ảnh markdown trên production** <!-- Updated: Red Team Session 1 - CSP -->:
  - `/api/v1/media/{id}/content` trả 302 sang URL ký sẵn của R2 (`MEDIA_ORIGIN`). CSP hiện tại
    `img-src 'self' blob:` (`infra/caddy/Caddyfile:16`, `infra/docker-compose.homelab.yml:82`) kiểm cả đích redirect,
    nên ảnh bị chặn — với bài cũ đã vậy, editor làm lỗi này lộ rõ (ảnh vừa chèn không hiện).
  - Stack E2E (web :8081, MinIO) có thể không bật CSP này, nên E2E xanh **không** chứng minh production đúng.
  - Sửa: thêm `{$MEDIA_ORIGIN}` / `${MEDIA_ORIGIN}` vào `img-src` ở cả hai file, sửa comment dòng 14 của Caddyfile.
    Đây là thay đổi hạ tầng tối thiểu, ngoài phạm vi api/web/db ban đầu (ghi ở câu hỏi mở của plan).
  - Header CSP của API (`default-src 'none'; img-src 'self' blob:`) chỉ áp cho response `/api/*`, không ảnh hưởng ảnh
    trong trang web; giữ nguyên.

## Steps

1. **`lesson-editor.spec.ts`** (`@serial`). Ở `beforeAll`: gọi `seedReset()`, admin tạo chặng `E2EMD` qua API
   (`POST /stages`) để có bản nháp v1 sạch. **`afterAll(seedReset)` bắt buộc**: spec tạo chặng, media và version
   mới; các spec sau (kể cả project a11y) dùng chung DB.
   - Paste/drop tổng hợp (E34 phần dán, E36): mặc định chạy trên chromium. Với webkit, chạy thử một lần; nếu sự kiện
     không tới được Lexical thì `test.skip(browserName === "webkit", "…")` kèm lý do, không để test xanh giả.
   - **E33. Thêm bài markdown qua trang soạn.**
     - "Thêm học liệu", chọn Markdown, nhập tiêu đề, bấm "Tiếp tục soạn". URL phải khớp `/lessons/new` và tiêu đề
       đã được điền sẵn.
     - Trong editor:
       - gõ `# Tiêu đề`;
       - dùng toolbar để in đậm và tạo danh sách;
       - chèn bảng;
       - chèn khối mã `go`;
       - tạo liên kết `https://example.com`.
     - Bấm Lưu. Kết quả: quay về trang chặng, có toast "Đã thêm học liệu.".
     - Kiểm DB: `markdown_source` chứa `# Tiêu đề`, `**`, `|`, ```` ```go ````.
   - **E34. Chặn định dạng ngoài GFM.**
     - **Đối chứng dương trước:** bôi chữ, nhấn `ControlOrMeta+B` → source có `**`. Chứng minh phím tắt tới được editor.
     - Bôi chữ khác rồi nhấn `ControlOrMeta+U`, sau đó lưu. Source không được chứa `<u>`.
     - Dán HTML qua `page.evaluate` bằng `ClipboardEvent` có `DataTransfer` với `text/html`:
       `<u>a</u><sup>2</sup><mark>m</mark><span style="color:red">s</span><h5>h</h5><a href="javascript:alert(1)">x</a><img src="https://evil.example/x.png">`.
       Kết quả mong đợi:
       - **đối chứng dương:** source chứa các chữ `a`, `2`, `m`, `s`, `h`, `x` đã dán (chứng minh paste được xử lý,
         không phải bị bỏ qua toàn bộ);
       - source không chứa `<u>`, `<sup>`, `==`, `style`, `javascript:`, `evil.example`;
       - `h5` bị hạ thành `####`.
   - **E35. Ảnh qua dialog bắt buộc alt.**
     - Mở `InsertImage`, chọn file PNG. Ô alt tự điền tên file bỏ đuôi.
     - Xóa alt thì nút "Chèn ảnh" bị disabled; nhập lại alt.
     - Chèn ảnh, lưu. Source khớp `!\[…\]\(/api/v1/media/[0-9a-f-]{36}/content\)`.
     - Ảnh trong editor tải được: `await expect.poll(() => img.evaluate(el => el.naturalWidth)).toBeGreaterThan(0)`.
     - Bảng `media` có bản ghi `kind='image'` ở trạng thái ready.
   - **E36. Dán ảnh lấy alt theo tên file.**
     - Dán `ClipboardEvent` có `DataTransfer.items.add(new File([png], "so-do-lop.png", { type: "image/png" }))`.
     - Sau khi lưu, source chứa `![so-do-lop](/api/v1/media/…/content)`.
     - Thêm bước kéo thả chuỗi URL ngoài (`DragEvent` với `text/uri-list`): không có ảnh nào được chèn. Assert sự kiện
       đã được xử lý (`event.defaultPrevented === true` trả về từ `page.evaluate`) để phân biệt "bị chặn" với "không
       tới editor".
   - **E36b. Lưu bị khóa khi ảnh đang tải.**
     - `page.route(<URL PUT ký sẵn của MinIO, lấy host/bucket từ env E2E>, …)` giữ request PUT đến khi test thả ra.
     - Dán ảnh: nút Lưu disabled với nhãn "Đang tải ảnh…"; thả request → nút Lưu bật lại; lưu → source có ảnh.
   - **E37. Xem trước.**
     - Chuyển sang tab "Xem trước". HTML có `<h1>`, `<table>`, `<pre><code class="language-go">`; liên kết có
       `rel` chứa `nofollow`; ảnh hiển thị: `naturalWidth > 0` (tải `/content` → 302 → 200).
     - Ghi chú trong báo cáo phase: xác nhận lại ảnh trên staging/homelab sau khi đổi CSP, vì stack E2E không chứng
       minh CSP production.
     - Quay lại tab Soạn thảo: nội dung còn nguyên và Undo vẫn chạy.
   - **E38. Server chặn ảnh ngoài.**
     - Gọi `admin.patch` trực tiếp với `markdownSource: "![a](https://x.example/a.png)"`. Kết quả 422 và thông
       điệp "Ảnh trong markdown phải tải lên qua hệ thống.".
     - Chuyển trang soạn sang chế độ "Mã nguồn", gõ ảnh ngoài rồi Lưu. Client chặn trước (`violations()`): `Alert`
       liệt kê URL ảnh, không có request PATCH, trang không rời đi.
     - Đăng nhập giáo viên (context riêng) gọi `POST /stages/markdown-preview` → 403.
   - **E39. Chặn rời trang.**
     - Sửa nội dung rồi bấm crumb "Chặng". `ConfirmDialog` "Rời trang?" hiện ra. Chọn "Ở lại" thì URL không đổi;
       chọn "Rời trang" thì điều hướng.
     - Sửa rồi reload: đăng ký `page.once("dialog", d => { seen = d.type(); d.dismiss(); })` **trước**, gọi
       `page.reload()` **không await** (reload bị giữ lại), `expect.poll(() => seen).toBe("beforeunload")`, rồi kiểm
       URL không đổi.
   - **E40. Round-trip nội dung seed (tiêu chí 7).**
     - Nhân bản v1 thành v2 nháp qua API cho **mọi** chặng seed có bài markdown (`DB`, `DS`, `GO`, `RE`, `WEB` — 7
       bài, gồm `re-state` có fence `jsx`).
     - Với **mọi** bài markdown của các v2:
       - mở trang sửa, chỉ đổi tiêu đề (thêm rồi xóa một ký tự cũng được), lưu;
       - kiểm `markdown_source` trong DB **bằng từng byte** với v1.
     - Với một bài, sửa nội dung: gõ một khoảng trắng cuối đoạn rồi Backspace, lưu.
       - `POST /stages/markdown-preview` cho source mới phải trả **cùng HTML** với source v1.
       - Mục đích: bằng chứng rằng chuẩn hoá của MDXEditor không đổi kết quả render.
     - Phát hành v2: `markdown_html` của v2 bằng v1 cho các bài không sửa nội dung.
   - **E41. Phiên bản đã phát hành.**
     - Mở thẳng URL `/lessons/:id/edit` của bài thuộc `DB` v1. Có `Alert` hướng dẫn nhân bản, không có editor và
       không có nút "Lưu".
   - **Hồi quy E2E cũ.** Chạy `E02` (thêm video qua dialog), `E06`, `E22` mà không sửa chúng. `E22` gửi `<img>` dạng
     HTML thô nên vẫn 201 (phase 1 đã phân tích).
2. **E42 trong `lesson-editor.spec.ts`:** `E42 trang soạn học liệu không có vi phạm axe serious/critical`.
   - Mở trang sửa một bài của bản nháp `E2EMD`, rồi chạy `runAxe(page)` như E32.
   - Ngoại lệ axe/hit-area chỉ lấy từ danh sách đã ghi trong báo cáo phase 3 (mỗi mục có lý do); không thêm ngoại lệ
     mới trong lúc làm phase 5.
   - Kiểm thêm:
     - textbox có tên "Nội dung bài học";
     - toolbar dùng phím mũi tên được (Radix Toolbar roving focus);
     - Tab đi qua tiêu đề, bắt buộc, tab list, toolbar, nội dung, rồi Lưu;
     - ở viewport 375px, các nút toolbar, điều khiển code-block và bảng có hit area ≥ 44px (dùng lại helper đo của
       E26 nếu có), trừ ngoại lệ đã ghi.
3. **Kiểm bundle:**
   - `pnpm build`, rồi kiểm có chunk riêng chứa `mdxeditor`/`lexical` (grep tên module trong `dist/assets/*.js`).
   - Chunk của route học viên (`features/learning`) và `index-*.js` **không** chứa chuỗi `lexical`/`mdxeditor`.
   - Ghi kích thước gzip của chunk editor vào báo cáo phase.
   - Không thêm script mới vào repo, trừ khi đây là bước lặp lại nhiều lần; khi đó đặt trong `scripts/` kèm comment.
4. **Render check:** chạy `make render-check`. Mục tiêu là đổi prose (bảng/hr/h4/del/img) không làm vỡ so sánh của
   trang `lesson` với prototype. Nếu ảnh chụp đổi vì style mới, xác nhận phần đổi chỉ nằm ở các phần tử mới thêm.
5. **CSP:** sửa `img-src` trong `infra/caddy/Caddyfile` và `infra/docker-compose.homelab.yml` như mục Context.
   Kiểm cú pháp: `docker compose -f infra/docker-compose.homelab.yml config >/dev/null` (với env mẫu) và
   `caddy validate --config infra/caddy/Caddyfile` nếu có caddy (hoặc qua image `caddy`).
6. **Docs (`docs/architecture.md`):**
   - Dòng 61: câu CSP thêm `img-src` (ảnh markdown đi qua `/content` rồi redirect tới `MEDIA_ORIGIN`).
   - Dòng ~190 (stages): thêm `POST /stages/markdown-preview` (admin, `{markdownSource} → {html}`, cùng renderer lúc
     phát hành) và luật "ảnh markdown phải là `/api/v1/media/{uuid}/content`; kiểm khi lưu nếu có source mới (422),
     phát hành vẫn sanitize".
   - Dòng ~191 (media): ghi `kind=image` được dùng bởi editor markdown.
   - Mục Frontend (~246–300): mô tả trang soạn và các thành phần chính:
     - route `stages/:stageId/versions/:versionId/lessons/{new,:lessonId/edit}`;
     - MDXEditor lazy, gfmGuard, upload-only image dialog, tab Xem trước;
     - quy ước "chỉ GFM + allowlist server".
   - Không đổi `docs/database.md`, vì không có thay đổi DB.
   - Sau khi sửa, kiểm lại link và tên file, route trong docs so với mã.

## Verification

```bash
docker compose -f infra/docker-compose.homelab.yml config >/dev/null   # cú pháp CSP homelab (cần env mẫu)
make e2e                                              # e2e + a11y trên stack full; mã thoát của Playwright
cd apps/web && pnpm exec playwright test -c e2e/playwright.config.ts --project=webkit e2e/lesson-editor.spec.ts  # tổ hợp phím Meta trên webkit
make check                                            # lint + test API/web
make render-check
```

## Risk

- **Sự kiện paste/drop tổng hợp không đi đúng đường xử lý như thao tác thật.** Playwright tạo `ClipboardEvent` thật
  trong trang, và Lexical lắng nghe `paste` trên contenteditable nên cách này dùng được. Nếu một trình duyệt chặn,
  dùng `page.keyboard.press("ControlOrMeta+V")` sau khi `context.grantPermissions(["clipboard-read","clipboard-write"])`
  và ghi clipboard (chỉ chromium).
- **E40 phụ thuộc nội dung seed.** Seed thay đổi sau này sẽ làm test kiểm lại round-trip. Đây là hành vi mong muốn.
- **Thời gian chạy E2E tăng.** Ước khoảng 2–3 phút (E40 mở 7 bài). Chấp nhận được.
- **CSP production chỉ kiểm được trên staging.** E2E không bật Caddy production; báo cáo phase ghi bước kiểm tay.

## Rollback

Xóa spec mới, bỏ khỏi `E2E_SPECS`, revert phần docs. Revert CSP chỉ khi gây lỗi khác (ảnh markdown sẽ lại bị chặn).
