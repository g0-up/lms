-- Quản trị viên Trần Minh Quân (u-admin của prototype/seed.js), đã đăng nhập và đổi mật khẩu.
INSERT INTO users (id, email, email_normalized, full_name, role, status, password_hash, must_change_password, last_login_at, last_active_at)
VALUES ('01990000-0000-7000-8000-000000000001', 'quan.tran@goup.vn', 'quan.tran@goup.vn', 'Trần Minh Quân', 'admin', 'active',
        '$argon2id$v=19$m=19456,t=2,p=1$hVgN0I6aZI+AWq7rW6j/XQ$i8SWRmwDmbK6kHOJBAyuz4EOO7oK2G/nLDOcK3Q/NHE', false, now() - interval '2 hours', now() - interval '1 hour')
ON CONFLICT (id) DO NOTHING;
