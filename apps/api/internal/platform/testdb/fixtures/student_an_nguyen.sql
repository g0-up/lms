-- Học viên Nguyễn Hoàng An (u-an của prototype/seed.js).
INSERT INTO users (id, email, email_normalized, full_name, role, status, password_hash, must_change_password, last_login_at, last_active_at)
VALUES ('01990000-0000-7000-8000-000000000003', 'an.nguyen@gmail.com', 'an.nguyen@gmail.com', 'Nguyễn Hoàng An', 'student', 'active',
        '$argon2id$v=19$m=19456,t=2,p=1$hVgN0I6aZI+AWq7rW6j/XQ$i8SWRmwDmbK6kHOJBAyuz4EOO7oK2G/nLDOcK3Q/NHE', false, now() - interval '5 hours', now() - interval '4 hours')
ON CONFLICT (id) DO NOTHING;
