-- File video/ảnh trên object storage; thời lượng video nằm ở lessons.duration_seconds.
CREATE TABLE media_files (
  id uuid CONSTRAINT pk_media_files PRIMARY KEY,
  kind text NOT NULL CONSTRAINT ck_media_files_kind CHECK (kind IN ('video', 'image')),
  storage_key text NOT NULL CONSTRAINT uq_media_files_storage_key UNIQUE,
  original_name text NOT NULL,
  content_type text NOT NULL,
  size_bytes bigint NOT NULL CONSTRAINT ck_media_files_size_bytes CHECK (size_bytes > 0),
  status text NOT NULL CONSTRAINT ck_media_files_status CHECK (status IN ('pending', 'ready')),
  uploaded_by uuid NOT NULL CONSTRAINT fk_media_files_uploaded_by REFERENCES users (id) ON DELETE RESTRICT,
  created_at timestamptz NOT NULL DEFAULT now(),
  ready_at timestamptz
);
CREATE INDEX ix_media_files_status_created ON media_files (status, created_at);
