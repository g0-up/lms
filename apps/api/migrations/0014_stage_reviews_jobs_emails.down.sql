-- Hoàn tác 0014. Khôi phục CHECK mẫu email cũ báo 23514 nếu outbox còn thư mẫu mới; khi đó rollback bằng bản sao lưu.
ALTER TABLE email_outbox
  DROP CONSTRAINT IF EXISTS ck_email_outbox_template,
  ADD CONSTRAINT ck_email_outbox_template CHECK (template IN ('invite', 'added', 'resend', 'password_reset'));
DROP TABLE IF EXISTS scheduled_job_runs;
DROP TABLE IF EXISTS stage_reviews;
