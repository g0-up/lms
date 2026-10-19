-- Hoàn tác 0011. Chỉ chạy được khi chưa có học liệu quiz/homework và media document: khôi phục CHECK cũ sẽ báo
-- 23514 nếu còn. Rollback thật trên môi trường có dữ liệu dùng bản sao lưu pg_dump trước khi migrate.
DROP TABLE IF EXISTS homework_attachments;
DROP TABLE IF EXISTS rubric_criteria;
DROP TABLE IF EXISTS homework_settings;
DROP TABLE IF EXISTS question_options;
DROP TABLE IF EXISTS questions;
DROP TABLE IF EXISTS quiz_settings;

ALTER TABLE media_files
  DROP CONSTRAINT IF EXISTS ck_media_files_kind,
  ADD CONSTRAINT ck_media_files_kind CHECK (kind IN ('video', 'image'));

ALTER TABLE lessons
  DROP CONSTRAINT IF EXISTS ck_lessons_type_content,
  ADD CONSTRAINT ck_lessons_type_content CHECK (
    (type = 'video' AND video_media_id IS NOT NULL AND markdown_source IS NULL)
    OR (type = 'markdown' AND markdown_source IS NOT NULL AND video_media_id IS NULL)
  ),
  DROP CONSTRAINT IF EXISTS ck_lessons_type,
  ADD CONSTRAINT ck_lessons_type CHECK (type IN ('video', 'markdown'));
