package pgerr

import (
	"errors"
	"net/http"
	"strings"

	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5/pgconn"

	"lms/api/internal/platform/apperr"
)

// Map dịch lỗi Postgres sang *apperr.Error. Đây là nơi DUY NHẤT dịch tên constraint, và chỉ dùng hằng trong
// constraints.go. Lỗi không phải *pgconn.PgError, hoặc mã Postgres không có nhánh, được trả nguyên.
// Không có mã lỗi tự định nghĩa: database không có function/trigger.
//
// pg.Detail và pg.Message có thể chứa giá trị cột nên chỉ nằm trong cause (để log), không vào thông điệp.
// Thêm constraint mà người dùng có thể chạm tới: thêm case ở đây cùng test.
func Map(err error) error {
	var pg *pgconn.PgError
	if !errors.As(err, &pg) {
		return err
	}
	switch pg.Code {
	case pgerrcode.UniqueViolation:
		return apperr.Wrap(err, http.StatusConflict, uniqueCode(pg.ConstraintName), uniqueMessage(pg.ConstraintName))
	case pgerrcode.ForeignKeyViolation:
		// Insert/update tham chiếu dòng không tồn tại: "Key (...)=(...) is not present in table ...".
		if strings.Contains(pg.Detail, "is not present in table") {
			return apperr.Wrap(err, http.StatusNotFound, apperr.CodeNotFound, "Không tìm thấy dữ liệu tham chiếu")
		}
		return apperr.Wrap(err, http.StatusConflict, apperr.CodeInUse, apperr.Message(apperr.CodeInUse))
	case pgerrcode.CheckViolation:
		return apperr.Wrap(err, http.StatusBadRequest, apperr.CodeValidationFailed, checkMessage(pg.ConstraintName))
	case pgerrcode.SerializationFailure, pgerrcode.DeadlockDetected:
		return apperr.Wrap(err, http.StatusConflict, apperr.CodeConflict, "Xung đột dữ liệu, vui lòng thử lại")
	}
	return err
}

func uniqueCode(constraint string) string {
	switch constraint {
	case UqStageVersionsOneDraft, UqCourseVersionsOneDraft:
		return apperr.CodeDraftExists
	default:
		return apperr.CodeConflict
	}
}

func uniqueMessage(constraint string) string {
	switch constraint {
	case UqStageVersionsOneDraft, UqCourseVersionsOneDraft:
		return apperr.Message(apperr.CodeDraftExists)
	case UqUsersEmailNormalized:
		return "Email đã được sử dụng"
	case UqStagesCode, UqCoursesCode, UqClassesCode:
		return "Mã đã tồn tại"
	case UqClassMembersClassUser:
		return "Học viên đã có trong lớp"
	case UqLessonsVersionKey, UqLessonsVersionPosition, UqCvsVersionStage, UqCvsVersionPosition:
		return "Dữ liệu bị trùng trong phiên bản"
	default:
		return apperr.Message(apperr.CodeConflict)
	}
}

func checkMessage(constraint string) string {
	switch constraint {
	case CkClassesDates:
		return "Ngày kết thúc phải sau ngày bắt đầu"
	case CkLessonsTypeContent:
		return "Bài học video cần tệp video, bài đọc cần nội dung Markdown"
	case CkLessonsDuration:
		return "Thời lượng không hợp lệ"
	case CkUsersEmailNormalized:
		return "Email không hợp lệ"
	default:
		return apperr.Message(apperr.CodeValidationFailed)
	}
}
