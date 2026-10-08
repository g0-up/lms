-- Ghi quy ước schema vào comment của schema public để thấy ngay trong \dn+.
COMMENT ON SCHEMA public IS 'LMS: enum = text + CHECK; id uuid v7 sinh ở ứng dụng; không có function, trigger, type hay extension do người dùng tạo; bất biến phiên bản ở tầng ứng dụng (docs/database.md)';
