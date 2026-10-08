-- Chặng DB v2 đang soạn, clone từ v1: bài db-table đã sửa nhưng chưa render (markdown_html NULL), db-index giữ nguyên.
-- Cần stage_db_published.
INSERT INTO stage_versions (id, stage_id, version_no, status, title, description, cloned_from_id, created_by)
VALUES ('01990000-0000-7000-8000-000000000112', '01990000-0000-7000-8000-000000000101', 2, 'draft', 'Database',
        'Nền tảng SQL và thiết kế bảng.', '01990000-0000-7000-8000-000000000111', '01990000-0000-7000-8000-000000000001')
ON CONFLICT (id) DO NOTHING;

INSERT INTO lessons (id, stage_version_id, lesson_key, position, title, type, required, markdown_source, markdown_html, duration_seconds)
VALUES
  ('01990000-0000-7000-8000-000000000131', '01990000-0000-7000-8000-000000000112', 'db-table', 1,
   'Thiết kế bảng và khóa', 'markdown', true,
   E'# Thiết kế bảng và khóa\n\nMỗi bảng cần một khóa chính và khóa ngoại khi tham chiếu.', NULL, 720),
  ('01990000-0000-7000-8000-000000000132', '01990000-0000-7000-8000-000000000112', 'db-index', 2,
   'Đọc thêm: chỉ mục', 'markdown', false,
   E'# Chỉ mục\n\nChỉ mục giúp truy vấn nhanh hơn.', E'<h1>Chỉ mục</h1>\n<p>Chỉ mục giúp truy vấn nhanh hơn.</p>\n', 300)
ON CONFLICT (id) DO NOTHING;
