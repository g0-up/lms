-- Khóa học BASIC "Lập trình cơ bản" v1 đã phát hành, gồm chặng DB v1.
-- Cần stage_db_published.
INSERT INTO courses (id, code, name, created_by)
VALUES ('01990000-0000-7000-8000-000000000201', 'BASIC', 'Lập trình cơ bản', '01990000-0000-7000-8000-000000000001')
ON CONFLICT (id) DO NOTHING;

INSERT INTO course_versions (id, course_id, version_no, status, title, description, published_at, created_by)
VALUES ('01990000-0000-7000-8000-000000000211', '01990000-0000-7000-8000-000000000201', 1, 'published', 'Lập trình cơ bản',
        'Lộ trình nhập môn lập trình.', now() - interval '59 days', '01990000-0000-7000-8000-000000000001')
ON CONFLICT (id) DO NOTHING;

INSERT INTO course_version_stages (course_version_id, stage_version_id, stage_id, position)
VALUES ('01990000-0000-7000-8000-000000000211', '01990000-0000-7000-8000-000000000111', '01990000-0000-7000-8000-000000000101', 1)
ON CONFLICT (course_version_id, stage_version_id) DO NOTHING;
