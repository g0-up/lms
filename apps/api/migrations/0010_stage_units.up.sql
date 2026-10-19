-- Bài học (stage_units) nhóm học liệu trong phiên bản chặng; học liệu đã có vào một bài học mặc định.
CREATE TABLE stage_units (
  id uuid CONSTRAINT pk_stage_units PRIMARY KEY,
  stage_version_id uuid NOT NULL CONSTRAINT fk_stage_units_stage_version REFERENCES stage_versions (id) ON DELETE CASCADE,
  -- Giữ qua các phiên bản để đối chiếu bài học khi nhân bản, như lesson_key.
  unit_key text NOT NULL CONSTRAINT ck_stage_units_key CHECK (unit_key ~ '^[a-z0-9][a-z0-9-]{1,39}$'),
  title text NOT NULL CONSTRAINT ck_stage_units_title CHECK (char_length(btrim(title)) BETWEEN 1 AND 120),
  position integer NOT NULL CONSTRAINT ck_stage_units_position CHECK (position >= 1),
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT uq_stage_units_version_key UNIQUE (stage_version_id, unit_key),
  CONSTRAINT uq_stage_units_version_position UNIQUE (stage_version_id, position) DEFERRABLE INITIALLY IMMEDIATE,
  -- Thừa về logic (id đã là PK) nhưng là đích bắt buộc cho FK ghép fk_lessons_unit.
  CONSTRAINT uq_stage_units_id_version UNIQUE (id, stage_version_id)
);
-- Tên bài học duy nhất trong phiên bản, không phân biệt hoa thường và khoảng trắng hai đầu.
CREATE UNIQUE INDEX uq_stage_units_version_title ON stage_units (stage_version_id, lower(btrim(title)));

-- Backfill: mỗi phiên bản đã có nhận một bài học "Nội dung chặng" chứa mọi học liệu, lessons.position giữ nguyên
-- nên thứ tự và % tiến độ không đổi. gen_random_uuid() (built-in, sinh uuid v4) chỉ dùng ở đây vì migration không
-- gọi được bộ sinh v7 của ứng dụng; id v4 không ảnh hưởng tính đúng. Câu UPDATE khóa bảng lessons trong lúc chạy:
-- chạy ngoài giờ cao điểm khi dữ liệu lớn.
ALTER TABLE lessons ADD COLUMN unit_id uuid;
INSERT INTO stage_units (id, stage_version_id, unit_key, title, position)
  SELECT gen_random_uuid(), sv.id, 'noi-dung-chang', 'Nội dung chặng', 1 FROM stage_versions sv;
UPDATE lessons l SET unit_id = u.id FROM stage_units u WHERE u.stage_version_id = l.stage_version_id;

-- lessons.position vẫn là thứ tự phẳng duy nhất trong phiên bản; ứng dụng giữ nó tăng theo (bài học, thứ tự trong
-- bài học), nên thứ tự trong mỗi bài học cũng duy nhất. FK ghép buộc bài học cùng phiên bản với học liệu.
ALTER TABLE lessons
  ALTER COLUMN unit_id SET NOT NULL,
  ADD CONSTRAINT fk_lessons_unit FOREIGN KEY (unit_id, stage_version_id)
    REFERENCES stage_units (id, stage_version_id) ON DELETE CASCADE;
CREATE INDEX ix_lessons_unit_id ON lessons (unit_id);
