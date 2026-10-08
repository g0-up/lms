package classes

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"lms/api/internal/domain"
	"lms/api/internal/features/identity"
	"lms/api/internal/platform/clock"
	"lms/api/internal/platform/db"
)

// noTx báo lỗi nếu use case mở transaction: dữ liệu sai phải bị chặn trước khi chạm DB.
type noTx struct{ t *testing.T }

func (n noTx) Transact(context.Context, func(db.Executor) error) error {
	n.t.Fatal("không được mở transaction")
	return nil
}

func validationService(t *testing.T) *Service {
	return NewService(Deps{Tx: noTx{t}, Clock: &clock.Fake{T: testNow}})
}

func TestCreateClassValidatesBeforeTransaction(t *testing.T) {
	valid := CreateClassCmd{
		Code: "basic04", Name: "Lập trình cơ bản 04", CourseVersionID: uuid.NewString(),
		StartDate: "2026-11-01", EndDate: "2027-03-01", TeacherID: uuid.NewString(),
	}
	tests := []struct {
		name  string
		patch func(*CreateClassCmd)
		want  error
	}{
		{"thiếu mã", func(c *CreateClassCmd) { c.Code = " " }, ErrClassFieldsRequired},
		{"thiếu tên", func(c *CreateClassCmd) { c.Name = "" }, ErrClassFieldsRequired},
		{"mã sai định dạng", func(c *CreateClassCmd) { c.Code = "Basic 04!" }, domain.ErrInvalidCode},
		{"thiếu ngày", func(c *CreateClassCmd) { c.EndDate = "" }, ErrDatesRequired},
		{"ngày sai định dạng", func(c *CreateClassCmd) { c.StartDate = "01/11/2026" }, ErrDateFormat},
		{"kết thúc trước bắt đầu", func(c *CreateClassCmd) { c.EndDate = "2026-10-01" }, ErrInvalidDates},
		{"thiếu giảng viên", func(c *CreateClassCmd) { c.TeacherID = "" }, ErrTeacherRequired},
		{"giảng viên sai id", func(c *CreateClassCmd) { c.TeacherID = "abc" }, ErrTeacherInvalid},
		{"phiên bản sai id", func(c *CreateClassCmd) { c.CourseVersionID = "abc" }, ErrVersionNotPublished},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := valid
			tt.patch(&cmd)
			if _, err := validationService(t).CreateClass(context.Background(), Actor{}, cmd); !errors.Is(err, tt.want) {
				t.Fatalf("CreateClass = %v, muốn %v", err, tt.want)
			}
		})
	}
}

func TestInviteRejectsInvalidEmailBeforeTransaction(t *testing.T) {
	_, err := validationService(t).Invite(context.Background(), Actor{}, uuid.New(), InviteCmd{Email: "khong-phai-email", FullName: "A"})
	if !errors.Is(err, identity.ErrInvalidEmail) {
		t.Fatalf("Invite = %v", err)
	}
}

func TestListTeachingClassesOnlyForTeachers(t *testing.T) {
	_, err := validationService(t).ListTeachingClasses(context.Background(), Actor{ID: uuid.New(), Role: domain.RoleAdmin})
	if !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("ListTeachingClasses = %v", err)
	}
}

func TestAuthorizeView(t *testing.T) {
	owner := uuid.New()
	tests := []struct {
		name  string
		actor Actor
		want  error
	}{
		{"quản trị xem mọi lớp", Actor{ID: uuid.New(), Role: domain.RoleAdmin}, nil},
		{"giảng viên phụ trách", Actor{ID: owner, Role: domain.RoleTeacher}, nil},
		{"giảng viên lớp khác", Actor{ID: uuid.New(), Role: domain.RoleTeacher}, ErrNotOwnClass},
		{"học viên", Actor{ID: owner, Role: domain.RoleStudent}, domain.ErrForbidden},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := authorizeView(tt.actor, owner); !errors.Is(err, tt.want) {
				t.Fatalf("authorizeView = %v, muốn %v", err, tt.want)
			}
		})
	}
	if ErrNotOwnClass.Error() != "Bạn chỉ xem được lớp mình phụ trách." {
		t.Fatalf("message = %q", ErrNotOwnClass.Error())
	}
}

func TestHandlerMapsValidationTo422(t *testing.T) {
	gin.SetMode(gin.TestMode)
	admin := func(*gin.Context) (uuid.UUID, domain.Role, bool) { return uuid.New(), domain.RoleAdmin, true }
	pass := func(c *gin.Context) { c.Next() }
	r := gin.New()
	NewHandler(validationService(t), admin).Register(r.Group("/api/v1"), Guards{Admin: pass, Staff: pass, Teacher: pass, InviteLimit: pass})

	tests := []struct {
		name, method, path, body string
		status                   int
		code                     string
	}{
		{"thiếu ngày", http.MethodPost, "/api/v1/classes",
			`{"code":"basic04","name":"Lớp","courseVersionId":"` + uuid.NewString() + `","teacherId":"` + uuid.NewString() + `"}`,
			http.StatusUnprocessableEntity, "VALIDATION_FAILED"},
		{"JSON hỏng", http.MethodPost, "/api/v1/classes", `{`, http.StatusBadRequest, ""},
		{"lọc trạng thái sai", http.MethodGet, "/api/v1/classes?status=archived", "", http.StatusUnprocessableEntity, "VALIDATION_FAILED"},
		{"includeDropped sai", http.MethodGet, "/api/v1/classes/" + uuid.NewString() + "/members?includeDropped=co", "",
			http.StatusUnprocessableEntity, "VALIDATION_FAILED"},
		{"email sai", http.MethodPost, "/api/v1/classes/" + uuid.NewString() + "/invitations", `{"email":"x","fullName":"A"}`,
			http.StatusUnprocessableEntity, "VALIDATION_FAILED"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			req := httptest.NewRequestWithContext(context.Background(), tt.method, tt.path, strings.NewReader(tt.body))
			req.Header.Set("Content-Type", "application/json")
			r.ServeHTTP(w, req)
			if w.Code != tt.status {
				t.Fatalf("status = %d, muốn %d: %s", w.Code, tt.status, w.Body.String())
			}
			if tt.code == "" {
				return
			}
			var env struct {
				Error struct {
					Code string `json:"code"`
				} `json:"error"`
			}
			if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil || env.Error.Code != tt.code {
				t.Fatalf("envelope = %s (%v)", w.Body.String(), err)
			}
		})
	}
}
