//go:build integration

package pgerr_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"lms/api/internal/platform/apperr"
	"lms/api/internal/platform/db/pgerr"
	"lms/api/internal/platform/testdb"
)

// TestMapRealPostgresErrors chạy câu lệnh vi phạm constraint thật và kiểm Map dịch đúng mã lỗi.
// Mỗi trường hợp dùng transaction riêng vì một lỗi làm hỏng transaction hiện tại.
func TestMapRealPostgresErrors(t *testing.T) {
	dbx := testdb.Open(t)
	testdb.Reset(t, dbx)

	const stageDSID = "01990000-0000-7000-8000-0000000001f1"
	cases := []struct {
		name   string
		setup  string // chạy trước câu lỗi
		query  string
		args   []any
		status int
		code   string
		msg    string
	}{
		{
			name: "second draft of a stage",
			query: `INSERT INTO stage_versions (id, stage_id, version_no, status, title, created_by)
				VALUES ('01990000-0000-7000-8000-0000000001f2', $1, 3, 'draft', 'Database', $2)`,
			args:   []any{testdb.StageDBID, testdb.AdminQuanTranID},
			status: 409, code: apperr.CodeDraftExists, msg: "Đã có bản nháp, hãy tiếp tục chỉnh sửa bản nháp đó",
		},
		{
			name: "course version stage pointing at a version of another stage",
			setup: `INSERT INTO stages (id, code, name, created_by) VALUES ('` + stageDSID + `', 'DS', 'Data structure', '` +
				testdb.AdminQuanTranID + `')`,
			query:  `INSERT INTO course_version_stages (course_version_id, stage_version_id, stage_id, position) VALUES ($1, $2, $3, 2)`,
			args:   []any{testdb.CourseBasicV1ID, testdb.StageDBV2ID, stageDSID},
			status: 404, code: apperr.CodeNotFound, msg: "Không tìm thấy dữ liệu tham chiếu",
		},
		{
			name:   "deleting a stage version used by a course",
			query:  `DELETE FROM stage_versions WHERE id = $1`,
			args:   []any{testdb.StageDBV1ID},
			status: 409, code: apperr.CodeInUse, msg: "Không thể xóa vì đang được sử dụng",
		},
		{
			name:   "archived status without archived_at",
			query:  `UPDATE stage_versions SET status = 'archived' WHERE id = $1`,
			args:   []any{testdb.StageDBV1ID},
			status: 400, code: apperr.CodeValidationFailed, msg: "Dữ liệu không hợp lệ",
		},
		{
			name:   "duplicate stage code",
			query:  `INSERT INTO stages (id, code, name, created_by) VALUES ($1, 'DB', 'Trùng mã', $2)`,
			args:   []any{stageDSID, testdb.AdminQuanTranID},
			status: 409, code: apperr.CodeConflict, msg: "Mã đã tồn tại",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			tx := testdb.Tx(t, dbx)
			testdb.Fixture(t, tx, "course_basic_published")
			testdb.Fixture(t, tx, "stage_db_draft")
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			if c.setup != "" {
				_, err := tx.ExecContext(ctx, c.setup)
				require.NoError(t, err)
			}
			_, err := tx.ExecContext(ctx, c.query, c.args...)
			require.Error(t, err)

			mapped := pgerr.Map(err)
			var ae *apperr.Error
			require.True(t, errors.As(mapped, &ae), "%v", err)
			require.Equal(t, c.status, ae.Status, "%v", err)
			require.Equal(t, c.code, ae.Code)
			require.Equal(t, c.msg, ae.Message)
		})
	}
}
