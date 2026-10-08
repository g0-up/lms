-- Chặng DB "Database" v1 đã phát hành với hai bài Markdown đã render HTML; lesson_media rỗng.
-- Cần admin_quan_tran.
INSERT INTO stages (id, code, name, created_by)
VALUES ('01990000-0000-7000-8000-000000000101', 'DB', 'Database', '01990000-0000-7000-8000-000000000001')
ON CONFLICT (id) DO NOTHING;

INSERT INTO stage_versions (id, stage_id, version_no, status, title, description, published_at, created_by)
VALUES ('01990000-0000-7000-8000-000000000111', '01990000-0000-7000-8000-000000000101', 1, 'published', 'Database',
        'Nền tảng SQL và thiết kế bảng.', now() - interval '60 days', '01990000-0000-7000-8000-000000000001')
ON CONFLICT (id) DO NOTHING;

INSERT INTO lessons (id, stage_version_id, lesson_key, position, title, type, required, markdown_source, markdown_html, duration_seconds)
VALUES
  ('01990000-0000-7000-8000-000000000121', '01990000-0000-7000-8000-000000000111', 'db-table', 1,
   'Thiết kế bảng và khóa', 'markdown', true,
   E'# Thiết kế bảng và khóa\n\nMỗi bảng cần một khóa chính.', E'<h1>Thiết kế bảng và khóa</h1>\n<p>Mỗi bảng cần một khóa chính.</p>\n', 720),
  ('01990000-0000-7000-8000-000000000122', '01990000-0000-7000-8000-000000000111', 'db-index', 2,
   'Đọc thêm: chỉ mục', 'markdown', false,
   E'# Chỉ mục\n\nChỉ mục giúp truy vấn nhanh hơn.', E'<h1>Chỉ mục</h1>\n<p>Chỉ mục giúp truy vấn nhanh hơn.</p>\n', 300)
ON CONFLICT (id) DO NOTHING;
