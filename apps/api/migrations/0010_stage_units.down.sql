-- Hoàn tác 0010: gỡ unit_id khỏi lessons trước khi xóa stage_units; học liệu giữ nguyên thứ tự phẳng.
DROP INDEX IF EXISTS ix_lessons_unit_id;
ALTER TABLE IF EXISTS lessons DROP CONSTRAINT IF EXISTS fk_lessons_unit;
ALTER TABLE IF EXISTS lessons DROP COLUMN IF EXISTS unit_id;
DROP TABLE IF EXISTS stage_units;
