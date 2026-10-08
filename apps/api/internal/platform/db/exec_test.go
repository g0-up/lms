//go:build integration

package db_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/require"

	"lms/api/internal/platform/db"
	"lms/api/internal/platform/db/pgerr"
	"lms/api/internal/platform/testdb"
)

func TestExecAffectOne(t *testing.T) {
	dbx := testdb.Open(t)
	testdb.Reset(t, dbx)
	tx := testdb.Tx(t, dbx)
	testdb.Fixture(t, tx, "stage_db_draft")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Đúng một dòng: sửa tiêu đề bài trong bản nháp.
	err := db.ExecAffectOne(ctx, tx,
		`UPDATE lessons SET title = $1 WHERE id = $2 AND stage_version_id IN (SELECT id FROM stage_versions WHERE status = 'draft')`,
		"Thiết kế bảng (sửa)", testdb.LessonDBTableV2ID)
	require.NoError(t, err)

	// Không dòng nào: cùng câu lệnh trên bài của bản đã phát hành.
	err = db.ExecAffectOne(ctx, tx,
		`UPDATE lessons SET title = $1 WHERE id = $2 AND stage_version_id IN (SELECT id FROM stage_versions WHERE status = 'draft')`,
		"Không được sửa", testdb.LessonDBTableV1ID)
	require.ErrorIs(t, err, db.ErrNoRowsAffected)

	err = db.ExecAffectOne(ctx, tx, `UPDATE lessons SET title = title WHERE false`)
	require.ErrorIs(t, err, db.ErrNoRowsAffected)

	// Nhiều hơn một dòng cũng là vi phạm hợp đồng.
	err = db.ExecAffectOne(ctx, tx, `UPDATE lessons SET title = title WHERE stage_version_id = $1`, testdb.StageDBV2ID)
	require.ErrorIs(t, err, db.ErrNoRowsAffected)
	require.ErrorContains(t, err, "chạm 2 dòng")

	// Lỗi Postgres trả nguyên, caller tự đưa qua pgerr.Map.
	err = db.ExecAffectOne(ctx, tx, `UPDATE lessons SET duration_seconds = -1 WHERE id = $1`, testdb.LessonDBTableV2ID)
	var pg *pgconn.PgError
	require.True(t, errors.As(err, &pg), "%v", err)
	require.Equal(t, pgerr.CkLessonsDuration, pg.ConstraintName)
}
