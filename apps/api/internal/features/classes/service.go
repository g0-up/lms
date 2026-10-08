package classes

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"lms/api/internal/domain"
	"lms/api/internal/features/courses"
	"lms/api/internal/features/identity"
	"lms/api/internal/features/mailer"
	"lms/api/internal/platform/audit"
	"lms/api/internal/platform/clock"
	"lms/api/internal/platform/db"
	"lms/api/internal/platform/ids"
)

// Giới hạn gửi lại lời mời cho mỗi thành viên.
const (
	resendLimit  = 3
	resendWindow = time.Hour
)

// Template bị thay thế khi xoay mật khẩu tạm: email cũ mang mật khẩu đã hết hiệu lực.
var supersededTemplates = []string{string(mailer.TemplateInvite), string(mailer.TemplateResend)}

// Actor là người gọi; Role quyết định phạm vi xem lớp, RequestID đi vào audit.
type Actor struct {
	ID        uuid.UUID
	Role      domain.Role
	RequestID string
}

// EmailLocker giữ khóa theo email chuẩn hóa tới hết tx (identity.PGLoginAttemptRepo hiện thực), để hai request mời
// hoặc gửi lại cùng email chạy tuần tự.
type EmailLocker interface {
	LockEmail(ctx context.Context, ex db.Executor, email domain.Email) error
}

// OutboxSuperseder hủy các email lời mời còn chờ gửi của một địa chỉ (mailer.PGOutboxRepo hiện thực).
type OutboxSuperseder interface {
	SupersedeQueued(ctx context.Context, ex db.Executor, toEmail string, templates []string, now time.Time) (int64, error)
}

// ProgressReader cho biết phần trăm hoàn thành trung bình của thành viên active theo lớp (feature learning hiện
// thực). Lớp vắng trong map → chưa có số liệu.
type ProgressReader interface {
	AvgPercentByClass(ctx context.Context, ex db.Executor, classIDs []uuid.UUID) (map[uuid.UUID]int, error)
}

// Service là use case của lớp, thành viên và lời mời. Mọi use case ghi chạy trong một transaction; mời và gửi lại
// ghi user (identity), thành viên, lời mời và outbox (mailer) trong cùng transaction đó.
type Service struct {
	db            db.Executor
	tx            db.Tx
	classes       ClassRepo
	members       MemberRepo
	invitations   InvitationRepo
	users         identity.UserReader
	provisioner   identity.UserProvisioner
	locker        EmailLocker
	versions      courses.VersionReader
	mail          mailer.Enqueuer
	outbox        OutboxSuperseder
	progress      ProgressReader
	clock         clock.Clock
	audit         audit.Recorder
	ids           ids.Generator
	publicBaseURL string
	staleDays     int
}

// Deps là phụ thuộc của Service; DB dùng cho truy vấn đọc ngoài transaction.
type Deps struct {
	DB            db.Executor
	Tx            db.Tx
	Classes       ClassRepo
	Members       MemberRepo
	Invitations   InvitationRepo
	Users         identity.UserReader
	Provisioner   identity.UserProvisioner
	Locker        EmailLocker
	Versions      courses.VersionReader
	Mail          mailer.Enqueuer
	Outbox        OutboxSuperseder
	Progress      ProgressReader
	Clock         clock.Clock
	Audit         audit.Recorder
	IDs           ids.Generator
	PublicBaseURL string
	StaleDays     int
}

// NewService nối phụ thuộc.
func NewService(d Deps) *Service {
	return &Service{
		db: d.DB, tx: d.Tx, classes: d.Classes, members: d.Members, invitations: d.Invitations,
		users: d.Users, provisioner: d.Provisioner, locker: d.Locker, versions: d.Versions,
		mail: d.Mail, outbox: d.Outbox, progress: d.Progress, clock: d.Clock, audit: d.Audit, ids: d.IDs,
		publicBaseURL: d.PublicBaseURL, staleDays: d.StaleDays,
	}
}

// CreateClassCmd là dữ liệu tạo lớp; ngày dạng YYYY-MM-DD, id dạng chuỗi như client gửi.
type CreateClassCmd struct {
	Code            string
	Name            string
	CourseVersionID string
	StartDate       string
	EndDate         string
	TeacherID       string
}

// UpdateClassCmd là bản vá cài đặt lớp; nil = giữ nguyên.
type UpdateClassCmd struct {
	Name            *string
	StartDate       *string
	EndDate         *string
	TeacherID       *string
	CourseVersionID *string
}

// TeachingClassRow là lớp của giảng viên kèm phần trăm hoàn thành trung bình (nil khi chưa có số liệu).
type TeachingClassRow struct {
	ClassListRow
	AvgPercent *int
}

// InviteCmd là dữ liệu mời; FullName chỉ bắt buộc khi email chưa có tài khoản.
type InviteCmd struct {
	Email    string
	FullName string
}

// InviteKind là kết quả mời cho UI chọn thông báo: invited = tài khoản vừa nhận mật khẩu tạm mới qua email,
// added = thêm tài khoản có sẵn vào lớp.
type InviteKind string

// Các kết quả mời.
const (
	InviteKindInvited InviteKind = "invited"
	InviteKindAdded   InviteKind = "added"
)

// InviteResult là kết quả mời; không có field mật khẩu.
type InviteResult struct {
	Kind   InviteKind
	Member MemberRow
}

// CreateClass tạo lớp nháp gắn phiên bản khóa học đã phát hành và giảng viên đang hoạt động.
func (s *Service) CreateClass(ctx context.Context, actor Actor, cmd CreateClassCmd) (*ClassDetail, error) {
	if strings.TrimSpace(cmd.Code) == "" || strings.TrimSpace(cmd.Name) == "" {
		return nil, ErrClassFieldsRequired
	}
	code, err := domain.ParseCode(cmd.Code, domain.ClassCode)
	if err != nil {
		return nil, err
	}
	dates, err := parseDateRange(cmd.StartDate, cmd.EndDate)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(cmd.TeacherID) == "" {
		return nil, ErrTeacherRequired
	}
	teacherID, err := ids.Parse(strings.TrimSpace(cmd.TeacherID))
	if err != nil {
		return nil, ErrTeacherInvalid
	}
	versionID, err := ids.Parse(strings.TrimSpace(cmd.CourseVersionID))
	if err != nil {
		return nil, ErrVersionNotPublished
	}
	now := s.clock.Now()
	var out *ClassDetail
	err = s.tx.Transact(ctx, func(tx db.Executor) error {
		cv, err := s.publishedVersion(ctx, tx, versionID, ErrVersionNotPublished)
		if err != nil {
			return err
		}
		teacher, err := s.activeTeacher(ctx, tx, teacherID)
		if err != nil {
			return err
		}
		c, err := NewClass(s.ids.New(), code, cmd.Name, cv, dates, teacher, actor.ID, now)
		if err != nil {
			return err
		}
		if err := s.classes.Create(ctx, tx, c); err != nil {
			return err
		}
		if err := s.record(ctx, tx, actor, audit.ActionClassCreated, c.ID(), nil,
			map[string]any{"code": c.Code().String(), "name": c.Name(), "status": c.Status().String()}); err != nil {
			return err
		}
		out, err = s.classes.Detail(ctx, tx, c.ID())
		return err
	})
	return out, err
}

// UpdateClass áp bản vá cài đặt lớp. Đổi phiên bản khóa học chỉ khi lớp còn nháp và chỉ sang bản đã phát hành.
func (s *Service) UpdateClass(ctx context.Context, actor Actor, id uuid.UUID, cmd UpdateClassCmd) (*ClassDetail, error) {
	now := s.clock.Now()
	var out *ClassDetail
	err := s.tx.Transact(ctx, func(tx db.Executor) error {
		c, err := s.classes.ByIDForUpdate(ctx, tx, id)
		if err != nil {
			return err
		}
		from, prevVersion := c.Status(), c.CourseVersionID()
		changed, err := s.applyPatch(ctx, tx, c, cmd)
		if err != nil {
			return err
		}
		if changed {
			if err := s.classes.Update(ctx, tx, c, from, now); err != nil {
				return err
			}
		}
		if c.CourseVersionID() != prevVersion {
			if err := s.record(ctx, tx, actor, audit.ActionClassCourseVersionChanged, c.ID(),
				map[string]any{"courseVersionId": prevVersion}, map[string]any{"courseVersionId": c.CourseVersionID()}); err != nil {
				return err
			}
		}
		out, err = s.classes.Detail(ctx, tx, c.ID())
		return err
	})
	return out, err
}

// applyPatch áp từng field có mặt; trả changed = có field được áp.
func (s *Service) applyPatch(ctx context.Context, tx db.Executor, c *Class, cmd UpdateClassCmd) (bool, error) {
	changed := false
	if cmd.CourseVersionID != nil {
		versionID, parseErr := ids.Parse(strings.TrimSpace(*cmd.CourseVersionID))
		if parseErr != nil || versionID != c.CourseVersionID() {
			// Lớp không còn nháp báo trước, kể cả khi id phiên bản sai.
			if c.Status() != domain.ClassDraft {
				return false, ErrClassNotDraft
			}
			if parseErr != nil {
				return false, ErrChangeToUnpublished
			}
			cv, err := s.publishedVersion(ctx, tx, versionID, ErrChangeToUnpublished)
			if err != nil {
				return false, err
			}
			if err := c.ChangeCourseVersion(cv); err != nil {
				return false, err
			}
			changed = true
		}
	}
	if cmd.Name != nil {
		if err := c.Rename(*cmd.Name); err != nil {
			return false, err
		}
		changed = true
	}
	if cmd.StartDate != nil || cmd.EndDate != nil {
		start, end := FormatDate(c.Dates().Start), FormatDate(c.Dates().End)
		if cmd.StartDate != nil {
			start = *cmd.StartDate
		}
		if cmd.EndDate != nil {
			end = *cmd.EndDate
		}
		dates, err := parseDateRange(start, end)
		if err != nil {
			return false, err
		}
		if err := c.Reschedule(dates); err != nil {
			return false, err
		}
		changed = true
	}
	if cmd.TeacherID != nil {
		if err := c.editable(); err != nil {
			return false, err
		}
		raw := strings.TrimSpace(*cmd.TeacherID)
		if raw == "" {
			return false, ErrTeacherRequired
		}
		teacherID, err := ids.Parse(raw)
		if err != nil {
			return false, ErrTeacherInvalid
		}
		teacher, err := s.activeTeacher(ctx, tx, teacherID)
		if err != nil {
			return false, err
		}
		if err := c.AssignTeacher(teacher); err != nil {
			return false, err
		}
		changed = true
	}
	return changed, nil
}

// Activate chuyển lớp nháp sang đang chạy.
func (s *Service) Activate(ctx context.Context, actor Actor, id uuid.UUID) (*ClassDetail, error) {
	return s.transition(ctx, actor, id, audit.ActionClassActivated, (*Class).Activate)
}

// End kết thúc lớp đang chạy; lớp đã kết thúc chỉ xem.
func (s *Service) End(ctx context.Context, actor Actor, id uuid.UUID) (*ClassDetail, error) {
	return s.transition(ctx, actor, id, audit.ActionClassEnded, (*Class).End)
}

func (s *Service) transition(ctx context.Context, actor Actor, id uuid.UUID, action string, apply func(*Class, time.Time) error) (*ClassDetail, error) {
	now := s.clock.Now()
	var out *ClassDetail
	err := s.tx.Transact(ctx, func(tx db.Executor) error {
		c, err := s.classes.ByIDForUpdate(ctx, tx, id)
		if err != nil {
			return err
		}
		from := c.Status()
		if err := apply(c, now); err != nil {
			return err
		}
		if err := s.classes.Update(ctx, tx, c, from, now); err != nil {
			return err
		}
		if err := s.record(ctx, tx, actor, action, c.ID(),
			map[string]any{"status": from.String()}, map[string]any{"status": c.Status().String()}); err != nil {
			return err
		}
		out, err = s.classes.Detail(ctx, tx, c.ID())
		return err
	})
	return out, err
}

// GetClass trả chi tiết lớp; giảng viên chỉ xem lớp mình phụ trách.
func (s *Service) GetClass(ctx context.Context, actor Actor, id uuid.UUID) (*ClassDetail, error) {
	d, err := s.classes.Detail(ctx, s.db, id)
	if err != nil {
		return nil, err
	}
	if err := authorizeView(actor, d.TeacherID); err != nil {
		return nil, err
	}
	return d, nil
}

// ListClasses liệt kê lớp cho admin.
func (s *Service) ListClasses(ctx context.Context, q ListQuery) ([]ClassListRow, error) {
	q.StaleCutoff = s.staleCutoff()
	return s.classes.List(ctx, s.db, q)
}

// ListTeachingClasses liệt kê lớp giảng viên phụ trách kèm số liệu theo dõi học viên.
func (s *Service) ListTeachingClasses(ctx context.Context, actor Actor) ([]TeachingClassRow, error) {
	if actor.Role != domain.RoleTeacher {
		return nil, domain.ErrForbidden
	}
	teacherID := actor.ID
	rows, err := s.classes.List(ctx, s.db, ListQuery{TeacherID: &teacherID, StaleCutoff: s.staleCutoff()})
	if err != nil {
		return nil, err
	}
	classIDs := make([]uuid.UUID, 0, len(rows))
	for _, r := range rows {
		classIDs = append(classIDs, r.ID)
	}
	var avg map[uuid.UUID]int
	if len(classIDs) > 0 {
		if avg, err = s.progress.AvgPercentByClass(ctx, s.db, classIDs); err != nil {
			return nil, err
		}
	}
	out := make([]TeachingClassRow, 0, len(rows))
	for _, r := range rows {
		row := TeachingClassRow{ClassListRow: r}
		if p, ok := avg[r.ID]; ok {
			row.AvgPercent = &p
		}
		out = append(out, row)
	}
	return out, nil
}

// ListMembers liệt kê thành viên lớp (ẩn dropped trừ khi includeDropped); giảng viên chỉ xem lớp mình phụ trách.
func (s *Service) ListMembers(ctx context.Context, actor Actor, classID uuid.UUID, includeDropped bool) ([]MemberRow, error) {
	c, err := s.classes.ByID(ctx, s.db, classID)
	if err != nil {
		return nil, err
	}
	if err := authorizeView(actor, c.TeacherID()); err != nil {
		return nil, err
	}
	return s.members.ListRows(ctx, s.db, classID, includeDropped)
}

// Invite mời học viên vào lớp theo FR-01 trong một transaction: khóa theo email, khóa lớp, quyết định nhánh bằng
// DecideInvite rồi ghi user/thành viên/lời mời/outbox. Mật khẩu tạm chỉ đi vào Message.Secret.
func (s *Service) Invite(ctx context.Context, actor Actor, classID uuid.UUID, cmd InviteCmd) (InviteResult, error) {
	// Email học viên được mời lưu và gửi ở dạng chuẩn hóa (chữ thường), để danh sách thành viên và hộp thư khớp nhau.
	email, err := domain.ParseEmail(strings.ToLower(cmd.Email))
	if err != nil {
		return InviteResult{}, identity.ErrInvalidEmail
	}
	now := s.clock.Now()
	var out InviteResult
	err = s.tx.Transact(ctx, func(tx db.Executor) error {
		if err := s.locker.LockEmail(ctx, tx, email); err != nil {
			return err
		}
		cls, err := s.classes.ByIDForUpdate(ctx, tx, classID)
		if err != nil {
			return err
		}
		existing, err := s.users.SnapshotByEmailForUpdate(ctx, tx, email)
		if err != nil {
			return err
		}
		var member *ClassMember
		if existing != nil {
			if member, err = s.members.ByClassAndUser(ctx, tx, classID, existing.ID); err != nil {
				return err
			}
		}
		decision, err := DecideInvite(cls, existing, member, cmd.FullName, now)
		if err != nil {
			return err
		}
		if decision == DecisionCreateUser {
			return s.inviteNewUser(ctx, tx, actor, cls, email, cmd.FullName, now, &out)
		}
		return s.inviteExisting(ctx, tx, actor, cls, existing, member, decision, now, &out)
	})
	return out, err
}

func (s *Service) inviteNewUser(ctx context.Context, tx db.Executor, actor Actor, cls *Class, email domain.Email, fullName string, now time.Time, out *InviteResult) error {
	u, tmp, created, err := s.provisioner.ProvisionStudent(ctx, tx, email, fullName, now)
	if err != nil {
		return err
	}
	if !created {
		return fmt.Errorf("classes: email %s vừa có tài khoản dù đang giữ khóa", email.String())
	}
	snap := u.Snapshot()
	m := NewMember(s.ids.New(), cls.ID(), snap.ID, now)
	if err := s.members.Create(ctx, tx, m); err != nil {
		return err
	}
	inv := outgoing{kind: InvitationInvite, secret: tmp.Reveal(), expiresAt: snap.TempPasswordExpiresAt}
	return s.finishInvite(ctx, tx, actor, cls, m, &snap, inv, InviteKindInvited, now, out)
}

func (s *Service) inviteExisting(ctx context.Context, tx db.Executor, actor Actor, cls *Class, u *identity.UserSnapshot, m *ClassMember, decision InviteDecision, now time.Time, out *InviteResult) error {
	kind := InviteKindAdded
	if decision == DecisionRejoin {
		from := m.Status()
		if err := m.Rejoin(); err != nil {
			return err
		}
		if err := s.members.Update(ctx, tx, m, from); err != nil {
			return err
		}
		// Học viên quay lại vẫn chưa đổi mật khẩu: gửi như nhánh invited (giữ hoặc xoay mật khẩu tạm).
		decision = DecisionAddActive
		if u.MustChangePassword {
			decision = DecisionAddInvitedRotate
			if tempPasswordValid(u, now) {
				decision = DecisionAddInvitedKeep
			}
		}
	} else {
		m = NewMember(s.ids.New(), cls.ID(), u.ID, now)
		if err := s.members.Create(ctx, tx, m); err != nil {
			return err
		}
		if decision == DecisionAddInvitedRotate {
			kind = InviteKindInvited
		}
	}
	inv := outgoing{kind: InvitationAdded}
	switch decision {
	case DecisionAddInvitedKeep:
		inv.usePreviousTempPassword = true
	case DecisionAddInvitedRotate:
		tmp, expires, err := s.rotateTemporaryPassword(ctx, tx, u.Email, now)
		if err != nil {
			return err
		}
		inv = outgoing{kind: InvitationInvite, secret: tmp.Reveal(), expiresAt: expires}
	}
	return s.finishInvite(ctx, tx, actor, cls, m, u, inv, kind, now, out)
}

func (s *Service) finishInvite(ctx context.Context, tx db.Executor, actor Actor, cls *Class, m *ClassMember, u *identity.UserSnapshot, inv outgoing, kind InviteKind, now time.Time, out *InviteResult) error {
	if err := s.send(ctx, tx, actor, cls, u, inv, now); err != nil {
		return err
	}
	if err := s.record(ctx, tx, actor, audit.ActionClassMemberInvited, cls.ID(), nil,
		map[string]any{"classId": cls.ID(), "userId": u.ID, "memberId": m.ID(), "kind": string(inv.kind)}); err != nil {
		return err
	}
	row, err := s.members.RowByID(ctx, tx, cls.ID(), m.ID())
	if err != nil {
		return err
	}
	*out = InviteResult{Kind: kind, Member: *row}
	return nil
}

// Resend cấp mật khẩu tạm mới cho thành viên chưa đổi mật khẩu và gửi lại lời mời; tối đa resendLimit lần mỗi
// resendWindow, đếm dưới khóa hàng thành viên.
func (s *Service) Resend(ctx context.Context, actor Actor, classID, memberID uuid.UUID) (*MemberRow, error) {
	// Đọc email trước để khóa theo email đứng đầu tx, cùng thứ tự khóa với Invite và đăng nhập.
	pre, err := s.members.ByID(ctx, s.db, classID, memberID)
	if err != nil {
		return nil, err
	}
	preUser, err := s.users.SnapshotByID(ctx, s.db, pre.UserID())
	if err != nil {
		return nil, err
	}
	if preUser == nil {
		return nil, ErrMemberNotFound
	}
	now := s.clock.Now()
	var out *MemberRow
	err = s.tx.Transact(ctx, func(tx db.Executor) error {
		if err := s.locker.LockEmail(ctx, tx, preUser.Email); err != nil {
			return err
		}
		m, err := s.members.ByIDForUpdate(ctx, tx, classID, memberID)
		if err != nil {
			return err
		}
		if !m.IsActive() {
			return ErrMemberAlreadyDropped
		}
		cls, err := s.classes.ByID(ctx, tx, classID)
		if err != nil {
			return err
		}
		if !cls.AcceptsInvitations() {
			return ErrClassEnded
		}
		n, err := s.invitations.CountResendSince(ctx, tx, classID, m.UserID(), now.Add(-resendWindow))
		if err != nil {
			return err
		}
		if n >= resendLimit {
			return ErrResendLimit
		}
		u, err := s.users.SnapshotByIDForUpdate(ctx, tx, m.UserID())
		if err != nil {
			return err
		}
		if u == nil {
			return ErrMemberNotFound
		}
		if u.Status == domain.UserDisabled {
			return ErrAccountDisabled
		}
		if !u.MustChangePassword {
			return ErrAlreadyActivated
		}
		tmp, expires, err := s.rotateTemporaryPassword(ctx, tx, u.Email, now)
		if err != nil {
			return err
		}
		inv := outgoing{kind: InvitationResend, secret: tmp.Reveal(), expiresAt: expires}
		if err := s.send(ctx, tx, actor, cls, u, inv, now); err != nil {
			return err
		}
		if err := s.record(ctx, tx, actor, audit.ActionClassInvitationResent, cls.ID(), nil,
			map[string]any{"classId": cls.ID(), "userId": u.ID, "memberId": m.ID()}); err != nil {
			return err
		}
		out, err = s.members.RowByID(ctx, tx, classID, memberID)
		return err
	})
	return out, err
}

// RemoveMember gỡ thành viên khỏi lớp (dropped); tiến độ học giữ nguyên để mời lại thì nối tiếp.
func (s *Service) RemoveMember(ctx context.Context, actor Actor, classID, memberID uuid.UUID) (*MemberRow, error) {
	now := s.clock.Now()
	var out *MemberRow
	err := s.tx.Transact(ctx, func(tx db.Executor) error {
		cls, err := s.classes.ByID(ctx, tx, classID)
		if err != nil {
			return err
		}
		if err := cls.editable(); err != nil {
			return err
		}
		m, err := s.members.ByIDForUpdate(ctx, tx, classID, memberID)
		if err != nil {
			return err
		}
		from := m.Status()
		if err := m.Drop(now); err != nil {
			return err
		}
		if err := s.members.Update(ctx, tx, m, from); err != nil {
			return err
		}
		if err := s.record(ctx, tx, actor, audit.ActionClassMemberDropped, cls.ID(),
			map[string]any{"status": from.String()},
			map[string]any{"classId": cls.ID(), "userId": m.UserID(), "memberId": m.ID(), "status": m.Status().String()}); err != nil {
			return err
		}
		out, err = s.members.RowByID(ctx, tx, classID, memberID)
		return err
	})
	return out, err
}

// outgoing là email lời mời cần gửi. secret rỗng với kind added.
type outgoing struct {
	kind                    InvitationKind
	secret                  string
	expiresAt               *time.Time
	usePreviousTempPassword bool
}

// send ghi email vào outbox và ghi một dòng invitations trỏ tới hàng outbox đó.
func (s *Service) send(ctx context.Context, tx db.Executor, actor Actor, cls *Class, u *identity.UserSnapshot, o outgoing, now time.Time) error {
	payload := map[string]any{
		"Name": u.Name, "ClassName": cls.Name(), "ClassCode": cls.Code().String(), "LoginURL": s.publicBaseURL + "/login",
	}
	template := mailer.TemplateAdded
	switch o.kind {
	case InvitationInvite:
		template = mailer.TemplateInvite
	case InvitationResend:
		template = mailer.TemplateResend
	case InvitationAdded:
	}
	if o.kind != InvitationAdded {
		if o.secret == "" || o.expiresAt == nil {
			return errors.New("classes: lời mời có mật khẩu tạm thiếu bí mật hoặc hạn dùng")
		}
		payload["Email"] = u.Email.String()
		payload["ExpiresAt"] = o.expiresAt.UTC()
	}
	if o.usePreviousTempPassword {
		payload["UsePreviousTempPassword"] = true
	}
	outboxID, err := s.mail.Enqueue(ctx, tx, mailer.Message{To: u.Email, Template: template, Payload: payload, Secret: o.secret})
	if err != nil {
		return err
	}
	return s.invitations.Create(ctx, tx, &Invitation{
		ID: s.ids.New(), ClassID: cls.ID(), UserID: u.ID, Kind: o.kind, EmailOutboxID: outboxID, InvitedBy: actor.ID, CreatedAt: now,
	})
}

// rotateTemporaryPassword xoay mật khẩu tạm của học viên (hàng user đã khóa trong tx) và hủy email lời mời cũ còn
// chờ gửi, trả mật khẩu mới và hạn dùng.
func (s *Service) rotateTemporaryPassword(ctx context.Context, tx db.Executor, email domain.Email, now time.Time) (identity.TemporaryPassword, *time.Time, error) {
	u, _, created, err := s.provisioner.ProvisionStudent(ctx, tx, email, "", now)
	if err != nil {
		return identity.TemporaryPassword{}, nil, err
	}
	if created {
		return identity.TemporaryPassword{}, nil, fmt.Errorf("classes: không tìm thấy học viên %s để xoay mật khẩu tạm", email.String())
	}
	tmp, err := s.provisioner.RotateTemporaryPassword(ctx, tx, u, now)
	if err != nil {
		return identity.TemporaryPassword{}, nil, err
	}
	if _, err := s.outbox.SupersedeQueued(ctx, tx, email.String(), supersededTemplates, now); err != nil {
		return identity.TemporaryPassword{}, nil, err
	}
	return tmp, u.Snapshot().TempPasswordExpiresAt, nil
}

func (s *Service) publishedVersion(ctx context.Context, tx db.Executor, id uuid.UUID, invalid error) (PublishedCourseVersion, error) {
	v, err := s.versions.PublishedVersion(ctx, tx, id)
	if errors.Is(err, courses.ErrVersionNotFound) || errors.Is(err, courses.ErrNotPublished) {
		return PublishedCourseVersion{}, invalid
	}
	if err != nil {
		return PublishedCourseVersion{}, err
	}
	return PublishedCourseVersion{ID: v.ID, CourseName: v.CourseName, VersionNo: v.VersionNo}, nil
}

func (s *Service) activeTeacher(ctx context.Context, tx db.Executor, id uuid.UUID) (ActiveTeacher, error) {
	t, err := s.users.ActiveTeacher(ctx, tx, id)
	if err != nil {
		return ActiveTeacher{}, err
	}
	if t == nil {
		return ActiveTeacher{}, ErrTeacherInvalid
	}
	return ActiveTeacher{ID: t.ID, Name: t.Name}, nil
}

func (s *Service) staleCutoff() time.Time {
	return s.clock.Now().Add(-time.Duration(s.staleDays) * 24 * time.Hour)
}

// record ghi audit với target là lớp.
func (s *Service) record(ctx context.Context, tx db.Executor, actor Actor, action string, classID uuid.UUID, before, after any) error {
	actorID := actor.ID
	return s.audit.Record(ctx, tx, audit.Entry{
		ActorID: &actorID, Action: action, TargetType: "class", TargetID: &classID, Before: before, After: after, RequestID: actor.RequestID,
	})
}

// authorizeView: admin xem mọi lớp, giảng viên chỉ lớp mình phụ trách, vai trò khác không xem được.
func authorizeView(actor Actor, teacherID uuid.UUID) error {
	switch actor.Role {
	case domain.RoleAdmin:
		return nil
	case domain.RoleTeacher:
		if actor.ID == teacherID {
			return nil
		}
		return ErrNotOwnClass
	default:
		return domain.ErrForbidden
	}
}

func parseDateRange(start, end string) (DateRange, error) {
	s, err := ParseDate(start)
	if err != nil {
		return DateRange{}, err
	}
	e, err := ParseDate(end)
	if err != nil {
		return DateRange{}, err
	}
	return NewDateRange(s, e)
}
