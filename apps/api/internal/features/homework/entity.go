// Package homework là feature hạn nộp theo lớp, bài nộp của học viên và chấm theo tiêu chí. Bài nộp là bất biến trừ
// status; mỗi lần chấm hoặc sửa điểm là một dòng submission_grades mới, dòng mới nhất là kết quả hiện hành.
package homework

import (
	"fmt"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"lms/api/internal/domain"
)

// Giới hạn của form nộp và form chấm.
const (
	MaxFiles             = 5
	maxRepoURLRunes      = 500
	maxCommitRefRunes    = 100
	maxCommentRunes      = 5000
	maxScoreCommentRunes = 1000
)

// repoURLPattern: repo public trên GitHub, GitLab hoặc Bitbucket dạng https://host/chủ/repo[/…]. Server chỉ kiểm định
// dạng, không gọi ra ngoài để xác minh repo public. Scheme viết thường để khớp ck_submissions_repo_url.
var (
	repoURLPattern   = regexp.MustCompile(`^https://(?i:(www\.)?(github\.com|gitlab\.com|bitbucket\.org))/[^/\s]+/[^/\s]+(/\S*)?$`)
	commitRefPattern = regexp.MustCompile(`^[A-Za-z0-9._/-]+$`)
)

// Lỗi của feature, thông điệp theo prototype và spec.
var (
	ErrNoContent          = domain.ErrInvalid.WithMsg("Nộp đường dẫn repo hoặc ít nhất một file.")
	ErrRepoURL            = domain.ErrInvalid.WithMsg("Đường dẫn repo phải là https trên GitHub, GitLab hoặc Bitbucket, ví dụ https://github.com/ten-ban/ten-repo.")
	ErrCommitNeedsRepo    = domain.ErrInvalid.WithMsg("Commit hoặc tag chỉ đi kèm đường dẫn repo.")
	ErrCommitRef          = domain.ErrInvalid.WithMsg("Commit hoặc tag tối đa 100 ký tự, chỉ gồm chữ, số và . _ / -.")
	ErrTooManyFiles       = domain.ErrInvalid.WithMsg(fmt.Sprintf("Tối đa %d file mỗi lần nộp.", MaxFiles))
	ErrDuplicateFile      = domain.ErrInvalid.WithMsg("Mỗi file chỉ nộp một lần.")
	ErrFileUnavailable    = domain.ErrInvalid.WithMsg("File không tồn tại, chưa tải lên xong hoặc đã được nộp. Tải lên lại.")
	ErrCommentRequired    = domain.ErrInvalid.WithMsg("Nhập nhận xét chung cho học viên.")
	ErrCommentTooLong     = domain.ErrInvalid.WithMsg(fmt.Sprintf("Nhận xét tối đa %d ký tự.", maxCommentRunes))
	ErrUnknownCriterion   = domain.ErrInvalid.WithMsg("Tiêu chí không thuộc bài tập này.")
	ErrInvalidDueAt       = domain.ErrInvalid.WithMsg("Hạn nộp không hợp lệ.")
	ErrLessonNotInClass   = domain.ErrNotFound.WithMsg("Bài tập không thuộc lớp này.")
	ErrSubmissionNotFound = domain.ErrNotFound.WithMsg("Không tìm thấy bài nộp.")
	ErrNotMember          = domain.ErrForbidden.WithMsg("Bạn không học lớp này.")
	ErrMemberDropped      = domain.ErrForbidden.WithMsg("Bạn không còn học lớp này.")
	ErrNotOwnGrade        = domain.ErrForbidden.WithMsg("Bạn chỉ chấm được bài của lớp mình phụ trách.")
	ErrNotOwnDeadline     = domain.ErrForbidden.WithMsg("Bạn chỉ đặt hạn nộp cho lớp mình phụ trách.")
	ErrClassNotStarted    = domain.ErrInvalidTransition.WithMsg("Lớp chưa bắt đầu.")
	ErrClassEndedSubmit   = domain.ErrInvalidTransition.WithMsg("Lớp đã kết thúc, không nộp bài được nữa.")
	ErrClassEndedDeadline = domain.ErrInvalidTransition.WithMsg("Lớp đã kết thúc, không đổi hạn nộp được.")
	ErrAlreadyPassed      = domain.ErrInvalidTransition.WithMsg("Bài đã Đạt nên không nộp lại được.")
	ErrNotLatest          = domain.ErrInvalidTransition.WithMsg("Chỉ chấm bài nộp mới nhất. Học viên đã nộp lần sau.")
	ErrConcurrentChange   = domain.ErrConflict.WithMsg("Xung đột dữ liệu, vui lòng thử lại")
)

// FieldErrors là lỗi validate của form theo trường (khóa là tên trường JSON, điểm từng tiêu chí là
// "scores.<criterionId>"). Error() là thông điệp đầu tiên theo thứ tự trên form; errors.Is(err, domain.ErrInvalid) đúng.
type FieldErrors struct {
	Fields map[string]string
	first  *domain.Error
}

func (e *FieldErrors) Error() string { return e.first.Msg }

// Unwrap trả lỗi nghiệp vụ đầu tiên để errors.Is/As đi xuyên qua.
func (e *FieldErrors) Unwrap() error { return e.first }

// add ghi lỗi của một trường; trường đã có lỗi giữ lỗi đầu.
func (e *FieldErrors) add(field string, err *domain.Error) {
	if e.Fields == nil {
		e.Fields = map[string]string{}
	}
	if _, ok := e.Fields[field]; ok {
		return
	}
	e.Fields[field] = err.Msg
	if e.first == nil {
		e.first = err
	}
}

// orNil trả nil khi không có lỗi nào (tránh interface error khác nil bọc con trỏ nil).
func (e *FieldErrors) orNil() error {
	if e.first == nil {
		return nil
	}
	return e
}

// SubmissionStatus là trạng thái bài nộp; chỉ cột này đổi sau khi nộp.
type SubmissionStatus string

// Các trạng thái bài nộp: chờ chấm, Đạt, Cần làm lại, bị thay bởi lần nộp sau khi đang chờ chấm.
const (
	StatusPending    SubmissionStatus = "pending"
	StatusPassed     SubmissionStatus = "passed"
	StatusRework     SubmissionStatus = "rework"
	StatusSuperseded SubmissionStatus = "superseded"
)

// Result là kết quả một lần chấm.
type Result string

// Các kết quả chấm.
const (
	ResultPassed Result = "passed"
	ResultRework Result = "rework"
)

// Label là nhãn hiển thị của kết quả (email, nhật ký).
func (r Result) Label() string {
	if r == ResultPassed {
		return "Đạt"
	}
	return "Cần làm lại"
}

// ResultOf: Đạt khi total/max >= passPercent/100, so bằng số nguyên (total*100 >= passPercent*max) nên không lệch ở
// biên ngưỡng.
func ResultOf(total, maxPoints, passPercent int) Result {
	if total*100 >= passPercent*maxPoints {
		return ResultPassed
	}
	return ResultRework
}

// SubmitInput là dữ liệu form nộp bài như client gửi.
type SubmitInput struct {
	RepoURL   string
	CommitRef string
	MediaIDs  []uuid.UUID
}

// Normalize cắt khoảng trắng repo và commit rồi kiểm theo quy tắc nộp: cần repo hoặc ít nhất một file; repo đúng
// định dạng; commit chỉ đi kèm repo; tối đa MaxFiles file, không trùng. Quyền sở hữu file kiểm ở service (cần DB).
func (in SubmitInput) Normalize() (SubmitInput, error) {
	out := SubmitInput{RepoURL: strings.TrimSpace(in.RepoURL), CommitRef: strings.TrimSpace(in.CommitRef), MediaIDs: in.MediaIDs}
	var fe FieldErrors
	if out.RepoURL == "" && len(out.MediaIDs) == 0 {
		fe.add("repoUrl", ErrNoContent)
	}
	if out.RepoURL != "" && (utf8.RuneCountInString(out.RepoURL) > maxRepoURLRunes || !repoURLPattern.MatchString(out.RepoURL)) {
		fe.add("repoUrl", ErrRepoURL)
	}
	switch {
	case out.CommitRef != "" && out.RepoURL == "":
		fe.add("commitRef", ErrCommitNeedsRepo)
	case out.CommitRef != "" && (utf8.RuneCountInString(out.CommitRef) > maxCommitRefRunes || !commitRefPattern.MatchString(out.CommitRef)):
		fe.add("commitRef", ErrCommitRef)
	}
	if len(out.MediaIDs) > MaxFiles {
		fe.add("mediaIds", ErrTooManyFiles)
	}
	seen := make(map[uuid.UUID]bool, len(out.MediaIDs))
	for _, id := range out.MediaIDs {
		if seen[id] {
			fe.add("mediaIds", ErrDuplicateFile)
		}
		seen[id] = true
	}
	return out, fe.orNil()
}

// NextStatusOnResubmit là quyết định khi học viên nộp lần mới với bài nộp mới nhất ở trạng thái latest: bài đã Đạt
// không nộp lại được; bài đang chờ chấm bị thay (superseded=true); Cần làm lại hoặc chưa nộp thì nộp bình thường.
func NextStatusOnResubmit(latest *SubmissionStatus) (supersede bool, err error) {
	if latest == nil {
		return false, nil
	}
	switch *latest {
	case StatusPassed:
		return false, ErrAlreadyPassed
	case StatusPending:
		return true, nil
	}
	return false, nil
}

// IsLate: nộp sau hạn khi có hạn và thời điểm nộp sau hạn (nộp đúng hạn không muộn).
func IsLate(dueAt *time.Time, submittedAt time.Time) bool {
	return dueAt != nil && submittedAt.After(*dueAt)
}

// Criterion là một tiêu chí chấm của bài tập.
type Criterion struct {
	ID          uuid.UUID
	Description string
	MaxPoints   int
}

// ScoreInput là điểm một tiêu chí như client gửi; Points nil là chưa nhập.
type ScoreInput struct {
	CriterionID uuid.UUID
	Points      *int
	Comment     string
}

// Score là điểm đã kiểm của một tiêu chí.
type Score struct {
	CriterionID uuid.UUID
	Points      int
	Comment     string
}

// Grade là kết quả một lần chấm đã kiểm, chưa gắn bài nộp.
type Grade struct {
	Scores      []Score // theo thứ tự tiêu chí
	TotalPoints int
	MaxPoints   int
	Result      Result
	Comment     string
}

// NewGrade kiểm form chấm theo tiêu chí của bài tập (đủ điểm mọi tiêu chí, mỗi điểm 0..max, nhận xét chung khác
// rỗng) rồi tính tổng, điểm tối đa và kết quả theo ngưỡng passPercent.
func NewGrade(criteria []Criterion, passPercent int, scores []ScoreInput, comment string) (Grade, error) {
	var fe FieldErrors
	byID := make(map[uuid.UUID]ScoreInput, len(scores))
	known := make(map[uuid.UUID]bool, len(criteria))
	for _, c := range criteria {
		known[c.ID] = true
	}
	for _, s := range scores {
		if !known[s.CriterionID] {
			fe.add("scores", ErrUnknownCriterion)
			continue
		}
		byID[s.CriterionID] = s
	}
	g := Grade{Scores: make([]Score, 0, len(criteria)), Comment: strings.TrimSpace(comment)}
	for _, c := range criteria {
		field := "scores." + c.ID.String()
		in, ok := byID[c.ID]
		switch {
		case !ok || in.Points == nil:
			fe.add(field, domain.ErrInvalid.WithMsg(fmt.Sprintf("Nhập điểm cho tiêu chí \"%s\".", c.Description)))
			continue
		case *in.Points < 0 || *in.Points > c.MaxPoints:
			fe.add(field, domain.ErrInvalid.WithMsg(fmt.Sprintf("Điểm tiêu chí \"%s\" phải từ 0 đến %d.", c.Description, c.MaxPoints)))
			continue
		}
		note := strings.TrimSpace(in.Comment)
		if utf8.RuneCountInString(note) > maxScoreCommentRunes {
			fe.add(field, domain.ErrInvalid.WithMsg(fmt.Sprintf("Nhận xét tiêu chí \"%s\" tối đa %d ký tự.", c.Description, maxScoreCommentRunes)))
			continue
		}
		g.Scores = append(g.Scores, Score{CriterionID: c.ID, Points: *in.Points, Comment: note})
		g.TotalPoints += *in.Points
		g.MaxPoints += c.MaxPoints
	}
	switch {
	case g.Comment == "":
		fe.add("comment", ErrCommentRequired)
	case utf8.RuneCountInString(g.Comment) > maxCommentRunes:
		fe.add("comment", ErrCommentTooLong)
	}
	if err := fe.orNil(); err != nil {
		return Grade{}, err
	}
	g.Result = ResultOf(g.TotalPoints, g.MaxPoints, passPercent)
	return g, nil
}

// MemberGate kiểm học viên được nộp bài trong lớp: phải là thành viên đang học của lớp active.
func MemberGate(isMember bool, member domain.MemberStatus, class domain.ClassStatus) error {
	switch {
	case !isMember:
		return ErrNotMember
	case member != domain.MemberActive:
		return ErrMemberDropped
	case class == domain.ClassDraft:
		return ErrClassNotStarted
	case class != domain.ClassActive:
		return ErrClassEndedSubmit
	}
	return nil
}

// Actor là người gọi; Role quyết định phạm vi lớp, RequestID đi vào audit.
type Actor struct {
	ID        uuid.UUID
	Role      domain.Role
	RequestID string
}

// staffCanAccessClass: admin mọi lớp, giảng viên chỉ lớp mình phụ trách (denied khi khác lớp), vai trò khác không.
// Cùng quy tắc với reports.classHeader.
func staffCanAccessClass(actor Actor, teacherID uuid.UUID, denied error) error {
	switch {
	case actor.Role == domain.RoleAdmin:
		return nil
	case actor.Role == domain.RoleTeacher && actor.ID == teacherID:
		return nil
	case actor.Role == domain.RoleTeacher:
		return denied
	}
	return domain.ErrForbidden
}
