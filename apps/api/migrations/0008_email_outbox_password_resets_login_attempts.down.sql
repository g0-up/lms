-- Hoàn tác 0008: gỡ FK của invitations trước khi xóa email_outbox.
DROP TABLE IF EXISTS login_attempts;
DROP TABLE IF EXISTS password_reset_tokens;
DROP INDEX IF EXISTS ix_invitations_email_outbox_id;
ALTER TABLE IF EXISTS invitations DROP CONSTRAINT IF EXISTS fk_invitations_email_outbox;
DROP TABLE IF EXISTS email_outbox;
