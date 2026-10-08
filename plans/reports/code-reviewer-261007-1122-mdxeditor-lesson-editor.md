# Review: trình soạn MDXEditor cho học liệu markdown

Kết quả: không có lỗi Critical hay High. Một lỗi Medium là thật và đã được sửa kèm test. Một rủi ro Medium được kiểm bằng test và không tái hiện. Các mục Low được giữ nguyên, có ghi lý do.

Phạm vi review gồm:
- API: luật ảnh khi lưu, `POST /stages/markdown-preview` và test.
- Web: trang soạn, `components/markdown-editor/*`, upload, schema, route và sanitizer `MarkdownContent`.
- E2E: `lesson-editor.spec.ts`.
- CSP: Caddyfile và compose homelab.
- `docs/architecture.md`.

## Đã xử lý

| Mức | Phát hiện | Kết quả |
|---|---|---|
| Medium | Lưu chậm, người dùng xác nhận "Rời trang", PATCH trả về sau đó: `navigate(backPath, {replace: true})` kéo họ quay lại trang chặng và mất mục lịch sử. | **Đã sửa.** `lesson-editor-page.tsx` dừng sau `await` khi trang đã unmount. Test "stays where the user went when a save finishes after they left" fail trước khi sửa và pass sau khi sửa. |
| Medium | Chưa kiểm chứng: chuyển Soạn thảo → Mã nguồn → Soạn thảo mà không sửa có thể phát `onChange`, khiến bài seed bị gửi lại `markdownSource` đã chuẩn hoá. | **Không tái hiện.** Test mới chạy round-trip trên cả 7 bài seed và `- a\n- b`, không có `onChange` nào. Test được giữ làm hàng rào hồi quy. |

## Giữ nguyên

- **Low.** Đăng xuất chủ động (về `/login`) không hỏi trước khi bỏ thay đổi chưa lưu. Đây là hệ quả của việc miễn chặn `/login` để redirect 401 hoạt động. Đây là quyết định sản phẩm; xem câu hỏi bên dưới.
- **Low.** Khi upload ảnh gặp 401, trang không chuyển về `/login` (người dùng chỉ thấy toast). Hành vi này giống upload video hiện có, không phải hồi quy.
- **Low.** Snapshot phiên bản có thể lấy từ cache. Rủi ro đã được giảm: `keepSource` giữ nguồn chưa sửa, `VERSION_IMMUTABLE` có xử lý, và ghi đè bản nháp theo kiểu "ghi sau thắng" là hành vi sẵn có.
- **Low.** Bộ đếm dung lượng chỉ là con số hiển thị gần đúng. Kiểm tra thật là `checkMarkdownSave` trên đúng body JSON, và server vẫn chặn ở 64 KB.
- **Low.** Luật link phía client (`SAFE_URL`) chặt hơn server: link tương đối như `foo.md` bị liệt kê là vi phạm khi sửa nội dung cũ. Chặt hơn thì an toàn; seed không có link loại này.
- **Ghi chú test.** Preview 403 cho người không phải admin chỉ có E2E E38 phủ. Route kế thừa guard của nhóm `/stages`, có test 401 ở router.

## Đã xác minh đúng

- `foreignImages` duyệt AST và so `Destination` với regex neo hai đầu. Luật chỉ chạy khi có `markdownSource` mới; preview dùng chung bluemonday với lúc phát hành.
- XSS:
  - HTML preview được lọc bằng bluemonday ở server và DOMPurify ở client;
  - `rel` bị ép thành `noopener noreferrer`;
  - link `javascript:` bị gỡ ở cả `validateUrl` lẫn transform;
  - `suppressHtmlProcessing` được bật.
- CSP `img-src 'self' blob: {$MEDIA_ORIGIN}` có ở cả Caddyfile và compose homelab.
- Upload:
  - tối đa 5 ảnh dán đang tải;
  - nút Lưu khóa khi còn ảnh đang tải;
  - upload bị huỷ khi unmount;
  - kiểm 64 KB trên body JSON thật.
- Không có plan ID, số phase hay mã finding trong code, comment hoặc tên test.

## Kiểm sau khi sửa

- Web: `corepack pnpm test` 567/567, lint, typecheck và `lint:design` sạch.
- API: `go vet` và test stages/seed/app pass.

## Câu hỏi còn mở

- Có muốn hỏi xác nhận khi người dùng chủ động đăng xuất lúc còn thay đổi chưa lưu không? Muốn vậy thì redirect 401 phải mang cờ riêng để chỉ miễn chặn trường hợp hết phiên.
