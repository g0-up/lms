---
title: Plan MDXEditor cho học liệu markdown
date: 2026-10-07
summary: "Lập plan 5 phase tích hợp MDXEditor (api/web, không đổi DB), red-team 15 finding: nhận 13, bác 2"
---

# Plan MDXEditor cho học liệu markdown

## What happened

- Lập plan hard mode tại `plans/261007-1015-mdxeditor-lesson-editor/`, gồm 5 phase:
  1. API: chặn ảnh ngoài khi lưu (422) và `POST /stages/markdown-preview`.
  2. Web foundation.
  3. Component `MarkdownEditor`.
  4. Trang soạn.
  5. E2E, a11y, bundle, CSP, docs.
- Không thay đổi database: `lessons.markdown_source` là `text`, đã đủ dùng.
- Red-team với 3 reviewer, 15 finding sau khi gộp trùng, đối chiếu file:dòng trước khi quyết. Kết quả: nhận 13, bác 2.

## Decision

- **Validation (người dùng chốt):**
  - trang soạn riêng;
  - chặn ảnh ngoài khi lưu;
  - alt bắt buộc trong dialog (ảnh dán/kéo thả lấy tên file);
  - có tab Xem trước, render bằng server.
- **Nguồn nội dung khi lưu:** chỉ một nguồn là `contentTouched` + `getMarkdown()`, không so với dữ liệu query đang sống. Lý do: refetch lúc focus có thể ghi đè source.
- **Kiểm phía client:** đo body JSON thật (≤ 64 KB) thay vì đếm byte của source. `violations()` chặn nội dung cũ, vì transform không chạy lúc mount.
- **Chặn rời trang:** blocker bỏ qua chuyển hướng về `/login` khi 401, và chỉ dùng ref `allowLeave` (không dùng `form.reset`).
- **CSP production:** `img-src 'self' blob:` chặn redirect `/content` → R2. Cần thêm `MEDIA_ORIGIN` vào `img-src`; đây là thay đổi infra nằm ngoài phạm vi api/web/db.
- **Bác:**
  - token phiên bản, vì last-write-wins là hành vi hiện có và là non-goal;
  - rate limit preview và trả details trong 422.

## Next steps

- Chạy `/ak:cook plans/261007-1015-mdxeditor-lesson-editor/plan.md`.
- Spike đầu phase 3 (~2 h) để xác nhận 4 điểm:
  - `onChange` ở chế độ Mã nguồn;
  - fence ngôn ngữ lạ;
  - placeholder khi upload;
  - cách gán alt.
- Kiểm tay CSP trên staging sau phase 5.

> Historical work record — not durable authority. Prefer docs/specs/ADRs for current decisions.
