-- Tài khoản người dùng và phiên đăng nhập phía server.
CREATE TABLE users (
  id uuid CONSTRAINT pk_users PRIMARY KEY,
  email text NOT NULL,
  email_normalized text NOT NULL,
  full_name text NOT NULL CONSTRAINT ck_users_full_name CHECK (length(btrim(full_name)) BETWEEN 1 AND 120),
  role text NOT NULL CONSTRAINT ck_users_role CHECK (role IN ('admin', 'teacher', 'student')),
  status text NOT NULL CONSTRAINT ck_users_status CHECK (status IN ('invited', 'active', 'disabled')),
  password_hash text NOT NULL,
  must_change_password boolean NOT NULL DEFAULT false,
  temp_password_expires_at timestamptz,
  last_login_at timestamptz,
  last_active_at timestamptz,
  disabled_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT uq_users_email_normalized UNIQUE (email_normalized),
  CONSTRAINT ck_users_email_normalized CHECK (email_normalized = lower(btrim(email_normalized))),
  CONSTRAINT ck_users_disabled_at CHECK ((status = 'disabled') = (disabled_at IS NOT NULL))
);
CREATE INDEX ix_users_role_status ON users (role, status);

-- token_hash là SHA-256 (32 byte) của token opaque; token gốc không bao giờ chạm DB.
CREATE TABLE sessions (
  id uuid CONSTRAINT pk_sessions PRIMARY KEY,
  user_id uuid NOT NULL CONSTRAINT fk_sessions_user REFERENCES users (id) ON DELETE RESTRICT,
  token_hash bytea NOT NULL CONSTRAINT uq_sessions_token_hash UNIQUE,
  created_at timestamptz NOT NULL DEFAULT now(),
  expires_at timestamptz NOT NULL,
  last_seen_at timestamptz NOT NULL DEFAULT now(),
  revoked_at timestamptz,
  ip inet,
  user_agent text
);
CREATE INDEX ix_sessions_user_id ON sessions (user_id);
CREATE INDEX ix_sessions_expires_at ON sessions (expires_at);
