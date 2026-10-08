-- Nhật ký thao tác; action không CHECK, danh sách chuẩn giữ ở platform/audit.
CREATE TABLE audit_logs (
  id uuid CONSTRAINT pk_audit_logs PRIMARY KEY,
  -- NULL cho hành động hệ thống (worker).
  actor_id uuid CONSTRAINT fk_audit_logs_actor REFERENCES users (id) ON DELETE RESTRICT,
  action text NOT NULL,
  target_type text NOT NULL,
  target_id uuid,
  before jsonb,
  after jsonb,
  request_id text,
  at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX ix_audit_logs_at ON audit_logs (at DESC);
CREATE INDEX ix_audit_logs_target ON audit_logs (target_type, target_id, at DESC);
CREATE INDEX ix_audit_logs_actor_at ON audit_logs (actor_id, at DESC);
