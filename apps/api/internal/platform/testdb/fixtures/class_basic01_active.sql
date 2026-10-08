-- Lớp basic01 "Lập trình cơ bản – khóa 1" đang chạy trên BASIC v1, giảng viên Lê Thu Hương, học viên Nguyễn Hoàng An.
-- Cần course_basic_published, teacher_huong_le, student_an_nguyen.
INSERT INTO classes (id, code, name, course_version_id, teacher_id, status, start_date, end_date, created_by)
VALUES ('01990000-0000-7000-8000-000000000301', 'basic01', 'Lập trình cơ bản – khóa 1', '01990000-0000-7000-8000-000000000211',
        '01990000-0000-7000-8000-000000000002', 'active', current_date - 58, current_date + 55, '01990000-0000-7000-8000-000000000001')
ON CONFLICT (id) DO NOTHING;

INSERT INTO class_members (id, class_id, user_id, status, joined_at)
VALUES ('01990000-0000-7000-8000-000000000311', '01990000-0000-7000-8000-000000000301', '01990000-0000-7000-8000-000000000003',
        'active', now() - interval '58 days')
ON CONFLICT (id) DO NOTHING;
