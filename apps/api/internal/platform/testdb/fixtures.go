//go:build integration

package testdb

import (
	"context"
	"embed"
	"fmt"
	"testing"
	"time"

	"lms/api/internal/platform/db"
)

// Id cố định của dữ liệu trong fixtures/*.sql (dạng uuid v7) để test tham chiếu thẳng.
const (
	AdminQuanTranID   = "01990000-0000-7000-8000-000000000001"
	TeacherHuongLeID  = "01990000-0000-7000-8000-000000000002"
	StudentAnNguyenID = "01990000-0000-7000-8000-000000000003"

	StageDBID         = "01990000-0000-7000-8000-000000000101"
	StageDBV1ID       = "01990000-0000-7000-8000-000000000111" // published
	StageDBV2ID       = "01990000-0000-7000-8000-000000000112" // draft, clone từ v1
	LessonDBTableV1ID = "01990000-0000-7000-8000-000000000121"
	LessonDBIndexV1ID = "01990000-0000-7000-8000-000000000122"
	LessonDBTableV2ID = "01990000-0000-7000-8000-000000000131" // markdown_html NULL
	LessonDBIndexV2ID = "01990000-0000-7000-8000-000000000132"

	CourseBasicID   = "01990000-0000-7000-8000-000000000201"
	CourseBasicV1ID = "01990000-0000-7000-8000-000000000211" // published, gồm DB v1

	ClassBasic01ID    = "01990000-0000-7000-8000-000000000301" // active
	MemberAnBasic01ID = "01990000-0000-7000-8000-000000000311"
)

// FixturePassword là mật khẩu của mọi user trong fixture; password_hash là argon2id với cùng tham số của seed.
const FixturePassword = "fixture-pass-2026" //nolint:gosec // mật khẩu của dữ liệu test, không phải bí mật

//go:embed fixtures/*.sql
var fixtureFS embed.FS

// fixtureDeps liệt kê fixture phải nạp trước; Fixture nạp phụ thuộc theo chiều sâu.
var fixtureDeps = map[string][]string{
	"admin_quan_tran":        nil,
	"teacher_huong_le":       nil,
	"student_an_nguyen":      nil,
	"stage_db_published":     {"admin_quan_tran"},
	"stage_db_draft":         {"stage_db_published"},
	"course_basic_published": {"stage_db_published"},
	"class_basic01_active":   {"course_basic_published", "teacher_huong_le", "student_an_nguyen"},
}

// Fixture chạy fixtures/<name>.sql (sau các fixture nó phụ thuộc) trên ex, thường là testdb.Tx. Mọi INSERT có
// ON CONFLICT (khóa chính) DO NOTHING nên nạp lại cùng fixture, hoặc hai fixture chung phụ thuộc, không lỗi.
func Fixture(t testing.TB, ex db.Executor, name string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	loaded := map[string]bool{}
	if err := loadFixture(ctx, ex, name, loaded); err != nil {
		t.Fatalf("testdb: fixture %s: %v", name, err)
	}
}

func loadFixture(ctx context.Context, ex db.Executor, name string, loaded map[string]bool) error {
	if loaded[name] {
		return nil
	}
	deps, ok := fixtureDeps[name]
	if !ok {
		return fmt.Errorf("không có fixture %q", name)
	}
	for _, dep := range deps {
		if err := loadFixture(ctx, ex, dep, loaded); err != nil {
			return err
		}
	}
	sqlText, err := fixtureFS.ReadFile("fixtures/" + name + ".sql")
	if err != nil {
		return err
	}
	if _, err := ex.ExecContext(ctx, string(sqlText)); err != nil {
		return fmt.Errorf("%s.sql: %w", name, err)
	}
	loaded[name] = true
	return nil
}
