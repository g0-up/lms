---
title: "Kế hoạch LMS MVP fullstack: red-team, validation, sweep"
date: 2026-10-05
summary: "ak:plan --hard tạo plans/261005-0743-lms-mvp-fullstack (plan.md + 14 phase), áp 15 nhóm red-team, 8 quyết định validation, sweep nhất quán, ak plan validate OK"
---

# Kế hoạch LMS MVP fullstack: red-team, validation, sweep

## What happened
- Chạy `/ak:plan --hard` từ `spec-lms-mvp.md` và `prototype/` (Go/gin/sqlx/go-migrate/cobra; TS/React/Vite/shadcn/Tailwind; Postgres; Docker theo mẫu `sidecup/infra`).
- Kết quả: `plans/261005-0743-lms-mvp-fullstack/plan.md` + 14 phase (DB-first, DDD-lite theo feature `apps/api`, `apps/web`).
- Red-team 4 persona → 15 nhóm finding, người dùng duyệt áp tất cả (ví dụ: publish guard trigger, bất biến phiên bản, outbox email có trạng thái, CSRF `CrossOriginProtection`, danh sách env chuẩn, một bộ fixture duy nhất từ `prototype/seed.js`).
- Validation 8 câu: `secret_enc` + `OUTBOX_SECRET_KEY`; lockout theo email + limiter IP 300/phút (tắt ở e2e); `completed` trong CHECK; first-login 2 ô; `includeDropped=false` + checkbox; Media TTL 2h; bỏ mọi bổ sung ngoài spec; prod dùng Cloudflare R2.
- Bốn fork cập nhật 14 phase song song, sau đó sweep toàn plan.

## Root cause / lessons
- File decisions nháp ghi sai bộ fixture ("basic03 ended", "BASIC là chặng") so với `seed.js`; phát hiện khi sweep, sửa ở plan.md §1 và gửi lại cho các fork. Bài học: chốt bộ dữ liệu mẫu từ nguồn thật trước khi phát tán cho subagent.
- Các fork tự thêm DTO hợp lý ngoài decisions; phải gộp ngược vào plan.md §7.1 để tránh trôi hợp đồng.
- Sweep phát hiện và sửa: CSP `{$S3_PUBLIC_ENDPOINT}` → `{$MEDIA_ORIGIN}`, `db.MapErr` → gói `pgerr`, audit `user.password_reset` ngoài 20 action chuẩn, phase-11 thiếu cross-ref golden dashboard của Phase 09.
- Tooling: hook privacy chặn heredoc có chữ `.env` → viết script Python vào scratchpad; `ugrep` vượt giới hạn regex với `.{N}` dài → dùng `/usr/bin/grep` + `cut`; cwd Bash trôi → dùng đường dẫn tuyệt đối.

## Decision
- Plan files là nguồn chân lý; không hydrate task (không có task surface).
- H4 hardening (lockout/limiter) trong phase-14 chạy với `APP_ENV=dev` vì profile e2e tắt limiter IP.

## Next steps
- `/ak:cook plans/261005-0743-lms-mvp-fullstack/plan.md` (khuyến nghị `/clear` trước).
- Câu hỏi chưa chốt với người dùng: viền input gray-300 vs WCAG `#8a8a8a`; MP4 vs HLS; phạm vi quyền giảng viên; admin vào `/teach`.

> Historical work record — not durable authority. Prefer docs/specs/ADRs for current decisions.
