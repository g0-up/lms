-- Chỉ compose dev: database riêng cho integration test (TEST_DATABASE_URL). Chạy một lần khi volume pgdata mới tạo.
CREATE DATABASE lms_test;
