package courses

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/google/uuid"

	"lms/api/internal/domain"
	"lms/api/internal/platform/apperr"
	"lms/api/internal/platform/audit"
	"lms/api/internal/platform/db"
)

// ApplyResult là kết quả áp dụng phiên bản chặng cho một khóa học: NewVersionNo khi thành công, Error khi khóa học
// đó không đổi.
type ApplyResult struct {
	CourseID     uuid.UUID
	CourseCode   string
	NewVersionNo *domain.VersionNo
	Error        *apperr.Error
}

// ApplyStageVersion áp dụng phiên bản chặng stageVersionID cho từng khóa học: clone bản published mới nhất, thay
// phiên bản của cùng chặng (giữ vị trí) rồi phát hành ngay. Mỗi khóa học một transaction riêng; thất bại của một
// khóa học không ảnh hưởng khóa học khác. Lỗi trả về chỉ là lỗi toàn cục (nguồn không hợp lệ, danh sách rỗng).
func (s *Service) ApplyStageVersion(ctx context.Context, actor Actor, stageVersionID uuid.UUID, courseIDs []uuid.UUID) ([]ApplyResult, error) {
	if len(courseIDs) == 0 {
		return nil, ErrNoCoursesSelected
	}
	refs, err := s.stageVersions.Refs(ctx, s.db, []uuid.UUID{stageVersionID})
	if err != nil {
		return nil, err
	}
	src, ok := refs[stageVersionID]
	if !ok {
		return nil, ErrStageVersionNotFound
	}
	if src.Status != domain.VersionPublished {
		return nil, ErrSourceNotPublished
	}

	seen := make(map[uuid.UUID]bool, len(courseIDs))
	results := make([]ApplyResult, 0, len(courseIDs))
	for _, courseID := range courseIDs {
		if seen[courseID] {
			continue
		}
		seen[courseID] = true
		res := ApplyResult{CourseID: courseID}
		no, err := s.applyToCourse(ctx, actor, src, courseID, &res.CourseCode)
		if err != nil {
			res.Error = appError(ctx, err)
		} else {
			res.NewVersionNo = &no
		}
		results = append(results, res)
	}
	return results, nil
}

// applyToCourse chạy một transaction cho một khóa học. Thứ tự ghi bắt buộc: chèn header draft → chèn chặng (câu có
// điều kiện draft) → chuyển trạng thái draft → published; không bao giờ chèn header đã published.
func (s *Service) applyToCourse(ctx context.Context, actor Actor, src StageVersionRef, courseID uuid.UUID, courseCode *string) (domain.VersionNo, error) {
	var newNo domain.VersionNo
	err := s.tx.Transact(ctx, func(tx db.Executor) error {
		if err := s.courses.LockForUpdate(ctx, tx, courseID); err != nil {
			return err
		}
		course, err := s.courses.ByID(ctx, tx, courseID)
		if err != nil {
			return err
		}
		*courseCode = course.Code
		draft, err := s.versions.DraftOf(ctx, tx, courseID)
		if err != nil {
			return err
		}
		latest, err := s.versions.LatestPublished(ctx, tx, courseID)
		if err != nil {
			return err
		}
		cur, err := applyPreconditions(latest, draft, src)
		if err != nil {
			return err
		}
		next, err := s.versions.NextVersionNo(ctx, tx, courseID)
		if err != nil {
			return err
		}
		v, err := latest.CloneAsDraft(s.ids.New(), next, actor.ID)
		if err != nil {
			return err
		}
		old := StageVersionRef{ID: cur.StageVersionID, StageID: cur.StageID}
		if err := v.ReplaceStageVersion(old, src); err != nil {
			return err
		}
		if err := s.versions.Create(ctx, tx, v); err != nil {
			return err
		}
		// Nhân bản, phát hành và áp dụng là một thao tác: published_at và ba dòng audit dùng chung một mốc.
		now := s.clock.Now()
		if err := s.publish(ctx, tx, v, now); err != nil {
			return err
		}
		if err := s.recordCloned(ctx, tx, actor, now, latest, v); err != nil {
			return err
		}
		if err := s.recordAt(ctx, tx, actor, now, audit.ActionCourseVersionPublished, "course_version", v.ID(),
			map[string]any{"courseId": courseID, "versionNo": v.VersionNo(), "stageVersionIds": v.StageVersionIDs()}); err != nil {
			return err
		}
		newNo = v.VersionNo()
		return s.recordAt(ctx, tx, actor, now, audit.ActionCourseStageVersionApplied, "course", courseID, map[string]any{
			"courseId": courseID, "fromVersionNo": latest.VersionNo(), "toVersionNo": v.VersionNo(),
			"stageId": src.StageID, "fromStageVersionId": cur.StageVersionID, "toStageVersionId": src.ID,
		})
	})
	if err != nil {
		return 0, err
	}
	return newNo, nil
}

// applyPreconditions là điều kiện áp dụng cho một khóa học (hàm thuần): không có bản nháp, đã có bản published,
// bản published mới nhất chứa chặng của nguồn và chưa dùng đúng phiên bản nguồn. Trả chặng sẽ bị thay.
func applyPreconditions(latest, draft *CourseVersion, src StageVersionRef) (CourseVersionStage, error) {
	if draft != nil {
		return CourseVersionStage{}, &ErrDraftExists{DraftID: draft.ID(), No: draft.VersionNo()}
	}
	if latest == nil {
		return CourseVersionStage{}, ErrNoPublishedVersion
	}
	cur, ok := latest.StageVersionFor(src.StageID)
	if !ok {
		return CourseVersionStage{}, ErrCourseLacksStage
	}
	if cur.StageVersionID == src.ID {
		return CourseVersionStage{}, ErrAlreadyUsingVersion
	}
	return cur, nil
}

// appError chuyển lỗi của một khóa học thành lỗi hiển thị theo dòng, cùng mã như khi gọi riêng từng thao tác. Lỗi
// 5xx chỉ trả thông điệp chung nên được log tại đây.
func appError(ctx context.Context, err error) *apperr.Error {
	var draft *ErrDraftExists
	if errors.As(err, &draft) {
		return apperr.Wrap(err, http.StatusConflict, apperr.CodeDraftExists, draft.Error())
	}
	var inUse *ErrInUse
	if errors.As(err, &inUse) {
		return apperr.Wrap(err, http.StatusConflict, apperr.CodeInUse, inUse.Error())
	}
	var de *domain.Error
	if errors.As(err, &de) && de.Kind == domain.KindInvalid {
		return apperr.Wrap(err, http.StatusUnprocessableEntity, apperr.CodeValidationFailed, de.Msg)
	}
	ae := apperr.FromDomain(err)
	if ae.Status >= http.StatusInternalServerError {
		slog.Default().ErrorContext(ctx, "courses: áp dụng phiên bản chặng thất bại", slog.Any("err", err))
	}
	return ae
}
