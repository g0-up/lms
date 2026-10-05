---
title: "Plan LMS MVP: bỏ function/trigger ở DB, chuyển bất biến phiên bản sang tầng ứng dụng"
date: 2026-10-05
summary: "Session 2 (D1): plan 261005-0743-lms-mvp-fullstack không còn 0010_immutability_triggers; bất biến published/draft/publish-guard thành hợp đồng domain + FOR UPDATE + SQL có điều kiện; cập nhật plan.md và phase 02/03/05/06/14"
---

# Plan LMS MVP: bỏ function/trigger ở DB, chuyển bất biến phiên bản sang tầng ứng dụng

## What happened
Người dùng yêu cầu rà lại `plans/261005-0743-lms-mvp-fullstack/plan.md` và `phase-02-database-schema.md`: không cho phép function và trigger ở tầng database, chuyển logic sang tầng ứng dụng.

Bản plan trước dựa vào migration `0010_immutability_triggers` (4 function PL/pgSQL, 6 trigger, SQLSTATE `LMS01`) để bảo vệ: header `published`/`archived` bất biến, bảng con chỉ ghi khi cha `draft`, chặn publish khi còn `markdown_html IS NULL`, tự điền/kiểm `course_version_stages.stage_id`. `pgerr.Map` (Phase 03) dịch `LMS01` → 409; Phase 05/06 test "trên trigger thật"; Phase 14 H5 chạy `immutability_check.sql` qua psql.

## Decision
D1 (ghi trong Validation Log Session 2 của plan.md): cấm `CREATE FUNCTION|PROCEDURE|TRIGGER|TYPE|EXTENSION` trong migration. DB chỉ còn ràng buộc khai báo.

Thay thế:
- `0010` bị bỏ, còn 9 migration; `0001_extensions_and_enums` → `0001_schema_conventions` (bỏ `pgcrypto`, chỉ `COMMENT ON SCHEMA`).
- `cvs_sync_stage_id()` → FK ghép `fk_cvs_stage_version (stage_version_id, stage_id) → stage_versions(id, stage_id)` + `uq_stage_versions_id_stage`; thêm `ck_*_archived_at`.
- Bất biến còn lại → hợp đồng ba lớp ở Go (Phase 02 mục "Bất biến phiên bản ở tầng ứng dụng"): domain (`ErrVersionImmutable`, `ErrNotRendered`), `ByIDForUpdate` trong `Transact`, SQL có điều kiện (`UPDATE ... WHERE status='draft'`, `INSERT ... SELECT ... WHERE status='draft'`, `DELETE ... USING header WHERE status='draft'`, transition `WHERE status=$from AND NOT EXISTS (markdown chưa render)`) qua helper mới `db.ExecAffectOne` (Phase 03).
- `pgerr.Map` bỏ nhánh `LMS01`; sổ constraint bỏ hằng tên trigger.
- Test: `migrations_test.go` chứng minh `pg_trigger`/`pg_proc`/`pg_type`/`pg_extension` rỗng và câu SQL hợp đồng trả 0/1 dòng đúng; Phase 05/06 thêm `TestImmutability_*` gọi repository trực tiếp bỏ qua service; Phase 14 H5 → `make hardening-immutability`.
- Giới hạn chấp nhận: SQL tay qua psql không bị chặn; quy tắc vận hành ghi ở `docs/database.md`, `docs/runbook.md`.

## Next steps
- Thực thi Phase 01 → 02 theo plan (`/ak:cook`); khi viết Phase 02 cần tạo `docs/database.md` với bảng bất biến ở tầng ứng dụng.
- Câu hỏi để session sau (chưa đổi): viền input WCAG 1.4.11, MP4 vs HLS, phạm vi quyền giảng viên, admin vào `/teach/*`.

> Historical work record — not durable authority. Prefer docs/specs/ADRs for current decisions.
