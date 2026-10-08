---
title: Hoàn tất trình soạn MDXEditor cho học liệu markdown
date: 2026-10-07
summary: "5 phase xong; E2E 11 bài mới + 49 bài toàn bộ xanh; sửa 2 lỗi thật (submit dialog lọt ra form trang, lưu xong kéo người dùng quay lại) và vi phạm axe"
---

# Hoàn tất trình soạn MDXEditor cho học liệu markdown

# Hoàn tất trình soạn MDXEditor cho học liệu markdown

## What happened

- Đã xong 5 phase của `plans/261007-1015-mdxeditor-lesson-editor/`:
  - API chặn ảnh ngoài khi lưu và thêm endpoint xem trước;
  - nền web, component `MarkdownEditor`, trang soạn;
  - E2E, a11y, bundle, CSP, docs.
- Kiểm chứng đã chạy:
  - E2E mới (E33–E42, E36b) 11/11; toàn bộ `test:e2e` 49 bài; render-check 17/17;
  - unit web 567; Go unit + integration xanh;
  - lexical/mdxeditor chỉ nằm trong chunk lazy `lesson-editor-page` (247 kB gzip).

## Lỗi thật tìm ra khi kiểm

- **Dialog chèn ảnh.** Submit trong dialog làm lưu luôn form trang soạn, rồi trang rời đi. Dialog được portal nên DOM nằm ngoài form trang, nhưng React vẫn bubble synthetic event theo cây component. Sửa bằng `stopPropagation` trong submit của dialog. Chỉ E2E bắt được lỗi này; unit test của dialog render nó không nằm trong form.
- **Lưu chậm.** Người dùng xác nhận "Rời trang" trong lúc PATCH còn chạy. Khi PATCH xong, `navigate(backPath, {replace})` kéo họ quay lại trang chặng. Sửa bằng ref theo dõi mount.
- **axe trên trang soạn.**
  - MDXEditor để trống tên ô bảng, vùng CodeMirror và nút thêm hàng/cột. `editor-a11y.ts` gắn nhãn, nhận nút theo tiền tố class CSS-module, nên phải kiểm lại khi nâng MDXEditor.
  - Màu token của theme CodeMirror và placeholder thiếu tương phản. Khối mã chuyển sang một màu.

## Lessons

- Với Lexical trong Playwright, chọn bằng Shift+Arrow ngay sau khi gõ nhanh hay bị lệch neo. Bật hoặc tắt định dạng tại con trỏ thì ổn định hơn.
- Ctrl+End có thể nhảy vào ô bảng. Muốn thêm đoạn mới thì neo vào một heading đã biết.
- Event dán và thả do test tự tạo không chạy hành động mặc định của trình duyệt, nên `defaultPrevented` không chứng minh được gì. Assert kết quả thì đáng tin hơn.
- Rủi ro reviewer nêu (round-trip source ↔ rich-text làm "chạm" nội dung) được bác bằng một test chạy trên 7 bài seed, thay vì đoán.

## Còn mở

- WebKit cục bộ thiếu thư viện hệ thống (cần sudo); CI có job webkit.
- golangci-lint cục bộ build bằng go1.26, module cần go1.27.
- CSP `img-src` trên production cần kiểm tay trên staging hoặc homelab.
- Đăng xuất chủ động không hỏi trước khi bỏ thay đổi chưa lưu. Đây là quyết định sản phẩm.

> Historical work record — not durable authority. Prefer docs/specs/ADRs for current decisions.
