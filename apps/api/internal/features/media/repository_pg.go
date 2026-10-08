package media

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"lms/api/internal/platform/db"
	"lms/api/internal/platform/db/pgerr"
)

// PGRepo là Repo trên Postgres.
type PGRepo struct{}

var (
	_ Repo   = PGRepo{}
	_ Reader = PGRepo{}
)

type mediaRecord struct {
	ID          uuid.UUID  `db:"id"`
	Kind        string     `db:"kind"`
	StorageKey  string     `db:"storage_key"`
	FileName    string     `db:"original_name"`
	ContentType string     `db:"content_type"`
	SizeBytes   int64      `db:"size_bytes"`
	Status      string     `db:"status"`
	UploadedBy  uuid.UUID  `db:"uploaded_by"`
	CreatedAt   time.Time  `db:"created_at"`
	ReadyAt     *time.Time `db:"ready_at"`
}

func (r mediaRecord) toEntity() *MediaFile {
	return &MediaFile{
		id: r.ID, storageKey: r.StorageKey, kind: Kind(r.Kind), fileName: r.FileName, contentType: r.ContentType,
		sizeBytes: r.SizeBytes, status: Status(r.Status), uploadedBy: r.UploadedBy, createdAt: r.CreatedAt, readyAt: r.ReadyAt,
	}
}

func (PGRepo) Create(ctx context.Context, ex db.Executor, m *MediaFile) error {
	_, err := ex.ExecContext(ctx, `
		INSERT INTO media_files (id, kind, storage_key, original_name, content_type, size_bytes, status, uploaded_by, created_at, ready_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`,
		m.id, string(m.kind), m.storageKey, m.fileName, m.contentType, m.sizeBytes, string(m.status), m.uploadedBy, m.createdAt, m.readyAt)
	if err != nil {
		return pgerr.Map(err)
	}
	return nil
}

func (PGRepo) Update(ctx context.Context, ex db.Executor, m *MediaFile) error {
	err := db.ExecAffectOne(ctx, ex, `UPDATE media_files SET status = $2, ready_at = $3 WHERE id = $1`,
		m.id, string(m.status), m.readyAt)
	if errors.Is(err, db.ErrNoRowsAffected) {
		return ErrMediaNotFound
	}
	if err != nil {
		return pgerr.Map(err)
	}
	return nil
}

func (PGRepo) ByID(ctx context.Context, ex db.Executor, id uuid.UUID) (*MediaFile, error) {
	var r mediaRecord
	err := sqlx.GetContext(ctx, ex, &r, `
		SELECT id, kind, storage_key, original_name, content_type, size_bytes, status, uploaded_by, created_at, ready_at
		FROM media_files WHERE id = $1`, id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrMediaNotFound
	}
	if err != nil {
		return nil, pgerr.Map(err)
	}
	return r.toEntity(), nil
}

func (PGRepo) CanUserAccess(ctx context.Context, ex db.Executor, mediaID, userID uuid.UUID) (bool, error) {
	var ok bool
	err := sqlx.GetContext(ctx, ex, &ok, `
		SELECT EXISTS (
		  SELECT 1 FROM lesson_media lm
		  JOIN lessons l ON l.id = lm.lesson_id
		  JOIN course_version_stages cvs ON cvs.stage_version_id = l.stage_version_id
		  JOIN classes c ON c.course_version_id = cvs.course_version_id
		  WHERE lm.media_id = $1
		    AND ( c.teacher_id = $2
		       OR EXISTS (SELECT 1 FROM class_members cm WHERE cm.class_id = c.id AND cm.user_id = $2
		                  AND cm.status = 'active' AND c.status IN ('active', 'ended')) )
		)`, mediaID, userID)
	if err != nil {
		return false, pgerr.Map(err)
	}
	return ok, nil
}
