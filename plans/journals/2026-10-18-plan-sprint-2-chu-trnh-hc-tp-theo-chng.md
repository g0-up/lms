---
title: "Plan Sprint 2: chu trình học tập theo chặng"
date: 2026-10-18
summary: 16 phase phủ mọi FR P0/P1 của spec-lms-v2; red-team nhận 15/17 phát hiện; validate 7 câu
---

# Plan Sprint 2: chu trình học tập theo chặng

## What happened

- Lập plan `plans/261018-2036-sprint2-learning-cycle/` (16 phase, ~32 ngày công) cho toàn bộ FR P0 và P1 của `spec-lms-v2.md`, lấy `prototype/` làm chuẩn UX và copy.
- Red-team bốn persona (bảo mật, vận hành, giả định, phạm vi): gộp 17 phát hiện, nhận 15, bỏ 2. Chi tiết ở `reports/red-team-review.md`.
- Các lỗi đáng giá nhất mà bản plan đầu bỏ sót, đều đã kiểm lại trong code:
  - Action audit mới phải có trong `knownActions` (`platform/audit/audit.go:32`), nếu không `PG.Record` từ chối và giao dịch chấm bài rollback.
  - Mailer cấm khóa payload `Link` (`mailer/service.go:16`); URL email dùng khóa riêng dựng từ `PublicBaseURL`.
  - "Tự nộp, commit rồi trả lỗi" bên trong `db.Transact` sẽ bị rollback; đổi sang trả `Outcome` rồi map lỗi sau commit.
  - Thứ tự khóa chung: `class_members` trước, rồi bản nộp hoặc lượt quiz; đổi trạng thái có điều kiện trạng thái cũ và `ExecAffectOne`.
  - Route có `:id` lớp và `:mid`/`:svid` phải ràng theo lớp, như `reports.MemberReport`, để tránh IDOR.
  - `complete-bar.tsx` chứa `LessonNav`; chỉ ẩn nút tích với quiz và homework.
  - Tab mặc định của giảng viên giữ `report` như prototype (`app.js:1394`).
  - Việc định kỳ chạy trong `Cleanups` mỗi giờ của worker, không thêm vòng quét mỗi lần poll.

## Decision

- Validate 7 câu với người dùng:
  - OQ-1: repo phải public; server chỉ kiểm định dạng URL.
  - OQ-2: homework hoàn thành khi bài mới nhất được chấm Đạt.
  - OQ-15: dữ liệu cũ vào một bài học "Nội dung chặng".
  - Chặng không có học liệu bắt buộc hiện "Đang học".
  - Bài của học viên đã rời lớp bị ẩn khỏi hàng chờ.
  - Nhãn bản nộp bị thay thế theo prototype.
  - Giữ các giả định nhỏ.
- Bỏ: tách migration expand/contract (prod một instance), job dọn file mồ côi (đã có giới hạn 10 file).

## Next steps

- Thực thi bằng `/ak:cook` từ phase 01; phase 08, 11 và 13 sửa `src/app/router.tsx` nên chạy tuần tự.
- Câu hỏi còn mở: có tự động kiểm repo public bằng request ẩn danh lúc nộp không.

> Historical work record — not durable authority. Prefer docs/specs/ADRs for current decisions.
