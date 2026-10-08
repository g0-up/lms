package pgerr_test

import (
	"database/sql"
	"errors"
	"fmt"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/require"

	"lms/api/internal/platform/apperr"
	"lms/api/internal/platform/db/pgerr"
)

func TestMap(t *testing.T) {
	const notPresent = `Key (stage_version_id, stage_id)=(0199, 0198) is not present in table "stage_versions".`
	cases := []struct {
		name   string
		pg     *pgconn.PgError
		status int
		code   string
		msg    string
	}{
		{"stage draft", &pgconn.PgError{Code: "23505", ConstraintName: pgerr.UqStageVersionsOneDraft}, 409, apperr.CodeDraftExists, "Đã có bản nháp, hãy tiếp tục chỉnh sửa bản nháp đó"},
		{"course draft", &pgconn.PgError{Code: "23505", ConstraintName: pgerr.UqCourseVersionsOneDraft}, 409, apperr.CodeDraftExists, "Đã có bản nháp, hãy tiếp tục chỉnh sửa bản nháp đó"},
		{"email", &pgconn.PgError{Code: "23505", ConstraintName: pgerr.UqUsersEmailNormalized}, 409, apperr.CodeConflict, "Email đã được sử dụng"},
		{"stage code", &pgconn.PgError{Code: "23505", ConstraintName: pgerr.UqStagesCode}, 409, apperr.CodeConflict, "Mã đã tồn tại"},
		{"course code", &pgconn.PgError{Code: "23505", ConstraintName: pgerr.UqCoursesCode}, 409, apperr.CodeConflict, "Mã đã tồn tại"},
		{"class code", &pgconn.PgError{Code: "23505", ConstraintName: pgerr.UqClassesCode}, 409, apperr.CodeConflict, "Mã đã tồn tại"},
		{"member", &pgconn.PgError{Code: "23505", ConstraintName: pgerr.UqClassMembersClassUser}, 409, apperr.CodeConflict, "Học viên đã có trong lớp"},
		{"lesson key", &pgconn.PgError{Code: "23505", ConstraintName: pgerr.UqLessonsVersionKey}, 409, apperr.CodeConflict, "Dữ liệu bị trùng trong phiên bản"},
		{"lesson position", &pgconn.PgError{Code: "23505", ConstraintName: pgerr.UqLessonsVersionPosition}, 409, apperr.CodeConflict, "Dữ liệu bị trùng trong phiên bản"},
		{"cvs stage", &pgconn.PgError{Code: "23505", ConstraintName: pgerr.UqCvsVersionStage}, 409, apperr.CodeConflict, "Dữ liệu bị trùng trong phiên bản"},
		{"cvs position", &pgconn.PgError{Code: "23505", ConstraintName: pgerr.UqCvsVersionPosition}, 409, apperr.CodeConflict, "Dữ liệu bị trùng trong phiên bản"},
		{"other unique", &pgconn.PgError{Code: "23505", ConstraintName: pgerr.UqSessionsTokenHash}, 409, apperr.CodeConflict, "Dữ liệu bị trùng"},
		{"fk delete", &pgconn.PgError{Code: "23503", ConstraintName: pgerr.FkClassesCourseVersion, Detail: `Key (id)=(0199) is still referenced from table "classes".`}, 409, apperr.CodeInUse, "Không thể xóa vì đang được sử dụng"},
		{"fk insert", &pgconn.PgError{Code: "23503", ConstraintName: pgerr.FkCvsStageVersion, Detail: notPresent}, 404, apperr.CodeNotFound, "Không tìm thấy dữ liệu tham chiếu"},
		{"check dates", &pgconn.PgError{Code: "23514", ConstraintName: pgerr.CkClassesDates}, 400, apperr.CodeValidationFailed, "Ngày kết thúc phải sau ngày bắt đầu"},
		{"check lesson content", &pgconn.PgError{Code: "23514", ConstraintName: pgerr.CkLessonsTypeContent}, 400, apperr.CodeValidationFailed, "Bài học video cần tệp video, bài đọc cần nội dung Markdown"},
		{"check duration", &pgconn.PgError{Code: "23514", ConstraintName: pgerr.CkLessonsDuration}, 400, apperr.CodeValidationFailed, "Thời lượng không hợp lệ"},
		{"check email", &pgconn.PgError{Code: "23514", ConstraintName: pgerr.CkUsersEmailNormalized}, 400, apperr.CodeValidationFailed, "Email không hợp lệ"},
		{"check other", &pgconn.PgError{Code: "23514", ConstraintName: pgerr.CkStageVersionsArchivedAt}, 400, apperr.CodeValidationFailed, "Dữ liệu không hợp lệ"},
		{"serialization", &pgconn.PgError{Code: "40001"}, 409, apperr.CodeConflict, "Xung đột dữ liệu, vui lòng thử lại"},
		{"deadlock", &pgconn.PgError{Code: "40P01"}, 409, apperr.CodeConflict, "Xung đột dữ liệu, vui lòng thử lại"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			wrapped := fmt.Errorf("repo: %w", c.pg)
			got := pgerr.Map(wrapped)
			var ae *apperr.Error
			require.True(t, errors.As(got, &ae))
			require.Equal(t, c.status, ae.Status)
			require.Equal(t, c.code, ae.Code)
			require.Equal(t, c.msg, ae.Message)
			require.ErrorIs(t, got, c.pg, "lỗi gốc giữ trong cause để log")
			require.NotContains(t, ae.Message, "0199", "không lộ giá trị cột")
		})
	}
}

func TestMapPassThrough(t *testing.T) {
	require.NoError(t, pgerr.Map(nil))
	require.Same(t, sql.ErrNoRows, pgerr.Map(sql.ErrNoRows))

	custom := &pgconn.PgError{Code: "LMS01", Message: "custom"}
	got := pgerr.Map(custom)
	require.Same(t, custom, got, "mã tự định nghĩa không được dịch")
	require.False(t, apperr.Is(got, apperr.CodeConflict))

	notNull := &pgconn.PgError{Code: "23502"}
	require.Same(t, notNull, pgerr.Map(notNull))
}
