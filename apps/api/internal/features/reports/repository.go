package reports

import (
	"context"
	"time"

	"github.com/google/uuid"

	"lms/api/internal/platform/db"
)

// ReportRepo đọc dữ liệu báo cáo một lớp.
type ReportRepo interface {
	// ClassHeader đọc lớp kèm khóa học và giảng viên; lớp không tồn tại → classes.ErrClassNotFound.
	ClassHeader(ctx context.Context, ex db.Executor, classID uuid.UUID) (ClassHeader, error)
	// StageHeaders là các chặng của phiên bản khóa học theo vị trí, kèm số học liệu bắt buộc.
	StageHeaders(ctx context.Context, ex db.Executor, courseVersionID uuid.UUID) ([]StageHeader, error)
	// Rows là các dòng báo cáo của lớp đã lọc và sắp xếp theo f tại thời điểm now; f phải hợp lệ.
	Rows(ctx context.Context, ex db.Executor, classID uuid.UUID, f Filter, now time.Time) ([]ReportRow, error)
	// MemberRow là dòng báo cáo của thành viên memberID thuộc lớp classID (mọi trạng thái); không thuộc → nil.
	MemberRow(ctx context.Context, ex db.Executor, classID, memberID uuid.UUID) (*ReportRow, error)
	// Summary đếm số liệu toàn lớp theo q.
	Summary(ctx context.Context, ex db.Executor, classID uuid.UUID, q SummaryQuery) (Summary, error)
}

// DashboardRepo đọc các số liệu tổng của dashboard.
type DashboardRepo interface {
	KPIs(ctx context.Context, ex db.Executor) (KPIs, error)
	// Hints đếm lớp nháp, học viên chưa đăng nhập và lời mời thất bại; OutdatedCourses do service điền.
	Hints(ctx context.Context, ex db.Executor) (Hints, error)
	// Classes là mọi lớp theo mã, AvgPercent do service điền.
	Classes(ctx context.Context, ex db.Executor) ([]ClassRow, error)
	// TargetLabels tra tên các đối tượng của nhật ký; đối tượng không còn tồn tại vắng trong map.
	TargetLabels(ctx context.Context, ex db.Executor, refs []TargetRef) (map[TargetRef]TargetInfo, error)
}
