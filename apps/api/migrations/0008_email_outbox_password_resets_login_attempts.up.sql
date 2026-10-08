-- Outbox email (worker render lúc gửi), token đặt lại mật khẩu và log lần đăng nhập.
CREATE TABLE email_outbox (
  id uuid CONSTRAINT pk_email_outbox PRIMARY KEY,
  to_email text NOT NULL,
  template text NOT NULL CONSTRAINT ck_email_outbox_template CHECK (template IN ('invite', 'added', 'resend', 'password_reset')),
  -- payload KHÔNG chứa bí mật; mật khẩu tạm hoặc token đặt lại chỉ nằm ở secret_enc.
  payload jsonb NOT NULL DEFAULT '{}'::jsonb,
  -- AES-256-GCM bằng OUTBOX_SECRET_KEY; đặt NULL khi sent hoặc failed lần cuối.
  secret_enc bytea,
  status text NOT NULL DEFAULT 'queued' CONSTRAINT ck_email_outbox_status CHECK (status IN ('queued', 'sending', 'sent', 'failed')),
  attempts integer NOT NULL DEFAULT 0 CONSTRAINT ck_email_outbox_attempts CHECK (attempts >= 0),
  run_at timestamptz NOT NULL DEFAULT now(),
  locked_until timestamptz,
  last_error text,
  sent_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now()
);
-- Claim theo run_at, gồm cả hàng sending bị kẹt (locked_until quá hạn).
CREATE INDEX ix_email_outbox_claim ON email_outbox (run_at) WHERE status IN ('queued', 'sending');

ALTER TABLE invitations
  ADD CONSTRAINT fk_invitations_email_outbox FOREIGN KEY (email_outbox_id) REFERENCES email_outbox (id) ON DELETE RESTRICT;
CREATE INDEX ix_invitations_email_outbox_id ON invitations (email_outbox_id);

CREATE TABLE password_reset_tokens (
  id uuid CONSTRAINT pk_password_reset_tokens PRIMARY KEY,
  user_id uuid NOT NULL CONSTRAINT fk_prt_user REFERENCES users (id) ON DELETE RESTRICT,
  token_hash bytea NOT NULL CONSTRAINT uq_prt_token_hash UNIQUE,
  expires_at timestamptz NOT NULL,
  used_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX ix_prt_user_id ON password_reset_tokens (user_id);

-- Log append-only khối lượng lớn nên dùng bigserial thay uuid; khóa đăng nhập tính theo email.
CREATE TABLE login_attempts (
  id bigserial CONSTRAINT pk_login_attempts PRIMARY KEY,
  email_normalized text NOT NULL,
  ip inet,
  succeeded boolean NOT NULL,
  attempted_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX ix_login_attempts_email_time ON login_attempts (email_normalized, attempted_at DESC);
