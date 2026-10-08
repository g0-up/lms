package identity

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jmoiron/sqlx"

	"lms/api/internal/domain"
	"lms/api/internal/platform/db"
	"lms/api/internal/platform/db/pgerr"
)

// ErrSessionNotFound là lỗi khi token không khớp phiên nào còn hiệu lực (hoặc user đã bị vô hiệu hóa).
var ErrSessionNotFound = errors.New("identity: không tìm thấy phiên")

// touchEvery là khoảng tối thiểu giữa hai lần ghi last_seen_at của một phiên.
const touchEvery = time.Minute

const userColumns = `u.id, u.email, u.full_name, u.role, u.status, u.password_hash, u.must_change_password,
	u.temp_password_expires_at, u.last_login_at, u.last_active_at, u.disabled_at, u.created_at, u.updated_at`

type userRow struct {
	ID           uuid.UUID  `db:"id"`
	Email        string     `db:"email"`
	FullName     string     `db:"full_name"`
	Role         string     `db:"role"`
	Status       string     `db:"status"`
	PasswordHash string     `db:"password_hash"`
	MustChange   bool       `db:"must_change_password"`
	TempExpires  *time.Time `db:"temp_password_expires_at"`
	LastLoginAt  *time.Time `db:"last_login_at"`
	LastActiveAt *time.Time `db:"last_active_at"`
	DisabledAt   *time.Time `db:"disabled_at"`
	CreatedAt    time.Time  `db:"created_at"`
	UpdatedAt    time.Time  `db:"updated_at"`
}

func (r userRow) toUser() (*User, error) {
	email, err := domain.ParseEmail(r.Email)
	if err != nil {
		return nil, fmt.Errorf("identity: email của user %s: %w", r.ID, err)
	}
	role, err := domain.ParseRole(r.Role)
	if err != nil {
		return nil, fmt.Errorf("identity: role của user %s: %w", r.ID, err)
	}
	status, err := domain.ParseUserStatus(r.Status)
	if err != nil {
		return nil, fmt.Errorf("identity: status của user %s: %w", r.ID, err)
	}
	return RehydrateUser(UserSnapshot{
		ID: r.ID, Email: email, Name: r.FullName, Role: role, Status: status,
		MustChangePassword: r.MustChange, TempPasswordExpiresAt: r.TempExpires,
		LastLoginAt: r.LastLoginAt, LastActiveAt: r.LastActiveAt, DisabledAt: r.DisabledAt,
		CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
	}, PasswordHashFromPHC(r.PasswordHash)), nil
}

// PGUserRepo là UserRepo trên Postgres.
type PGUserRepo struct{}

var _ UserRepo = PGUserRepo{}

// Create chèn user; email trùng → ErrEmailTaken.
func (PGUserRepo) Create(ctx context.Context, ex db.Executor, u *User) error {
	s := u.Snapshot()
	_, err := ex.ExecContext(ctx, `
		INSERT INTO users (id, email, email_normalized, full_name, role, status, password_hash, must_change_password,
		  temp_password_expires_at, last_login_at, last_active_at, disabled_at, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)`,
		s.ID, s.Email.Display(), s.Email.String(), s.Name, string(s.Role), string(s.Status), u.password.PHC(),
		s.MustChangePassword, s.TempPasswordExpiresAt, s.LastLoginAt, s.LastActiveAt, s.DisabledAt, s.CreatedAt, s.UpdatedAt)
	if err != nil {
		var pg *pgconn.PgError
		if errors.As(err, &pg) && pg.ConstraintName == pgerr.UqUsersEmailNormalized {
			return ErrEmailTaken
		}
		return pgerr.Map(fmt.Errorf("identity: tạo user: %w", err))
	}
	return nil
}

// Update ghi mọi field có thể đổi của user theo id.
func (PGUserRepo) Update(ctx context.Context, ex db.Executor, u *User) error {
	s := u.Snapshot()
	err := db.ExecAffectOne(ctx, ex, `
		UPDATE users SET full_name = $2, status = $3, password_hash = $4, must_change_password = $5,
		  temp_password_expires_at = $6, last_login_at = $7, last_active_at = $8, disabled_at = $9, updated_at = $10
		WHERE id = $1`,
		s.ID, s.Name, string(s.Status), u.password.PHC(), s.MustChangePassword, s.TempPasswordExpiresAt,
		s.LastLoginAt, s.LastActiveAt, s.DisabledAt, s.UpdatedAt)
	if errors.Is(err, db.ErrNoRowsAffected) {
		return ErrUserNotFound
	}
	if err != nil {
		return pgerr.Map(fmt.Errorf("identity: cập nhật user %s: %w", s.ID, err))
	}
	return nil
}

// ByID đọc user theo id.
func (PGUserRepo) ByID(ctx context.Context, ex db.Executor, id uuid.UUID) (*User, error) {
	return getUser(ctx, ex, `SELECT `+userColumns+` FROM users u WHERE u.id = $1`, id)
}

// ByIDForUpdate đọc và khóa hàng user tới hết transaction.
func (PGUserRepo) ByIDForUpdate(ctx context.Context, ex db.Executor, id uuid.UUID) (*User, error) {
	return getUser(ctx, ex, `SELECT `+userColumns+` FROM users u WHERE u.id = $1 FOR UPDATE`, id)
}

// ByEmail đọc user theo email chuẩn hóa.
func (PGUserRepo) ByEmail(ctx context.Context, ex db.Executor, e domain.Email) (*User, error) {
	return getUser(ctx, ex, `SELECT `+userColumns+` FROM users u WHERE u.email_normalized = $1`, e.String())
}

// ListByRole liệt kê user theo vai trò (và trạng thái nếu có), sắp theo họ tên.
func (PGUserRepo) ListByRole(ctx context.Context, ex db.Executor, role domain.Role, status *domain.UserStatus) ([]*User, error) {
	var st *string
	if status != nil {
		v := string(*status)
		st = &v
	}
	var rows []userRow
	if err := sqlx.SelectContext(ctx, ex, &rows, `
		SELECT `+userColumns+` FROM users u
		WHERE u.role = $1 AND ($2::text IS NULL OR u.status = $2)
		ORDER BY u.full_name, u.id`, string(role), st); err != nil {
		return nil, fmt.Errorf("identity: liệt kê user: %w", err)
	}
	out := make([]*User, 0, len(rows))
	for _, r := range rows {
		u, err := r.toUser()
		if err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, nil
}

var _ UserReader = PGUserRepo{}

// SnapshotByID đọc user theo id; không có → nil, nil.
func (PGUserRepo) SnapshotByID(ctx context.Context, ex db.Executor, id uuid.UUID) (*UserSnapshot, error) {
	return getSnapshot(ctx, ex, `SELECT `+userColumns+` FROM users u WHERE u.id = $1`, id)
}

// SnapshotByEmail đọc user theo email chuẩn hóa; không có → nil, nil.
func (PGUserRepo) SnapshotByEmail(ctx context.Context, ex db.Executor, e domain.Email) (*UserSnapshot, error) {
	return getSnapshot(ctx, ex, `SELECT `+userColumns+` FROM users u WHERE u.email_normalized = $1`, e.String())
}

// SnapshotByIDs đọc các user có id trong ids bằng một truy vấn; id không tồn tại vắng mặt trong map.
func (PGUserRepo) SnapshotByIDs(ctx context.Context, ex db.Executor, ids []uuid.UUID) (map[uuid.UUID]UserSnapshot, error) {
	out := make(map[uuid.UUID]UserSnapshot, len(ids))
	if len(ids) == 0 {
		return out, nil
	}
	strs := make([]string, len(ids))
	for i, id := range ids {
		strs[i] = id.String()
	}
	var rows []userRow
	if err := sqlx.SelectContext(ctx, ex, &rows, `SELECT `+userColumns+` FROM users u WHERE u.id = ANY($1::uuid[])`, strs); err != nil {
		return nil, fmt.Errorf("identity: đọc user theo danh sách id: %w", err)
	}
	for _, r := range rows {
		u, err := r.toUser()
		if err != nil {
			return nil, err
		}
		out[u.ID()] = u.Snapshot()
	}
	return out, nil
}

// SnapshotByIDForUpdate đọc và khóa hàng user tới hết transaction; không có → nil, nil.
func (PGUserRepo) SnapshotByIDForUpdate(ctx context.Context, ex db.Executor, id uuid.UUID) (*UserSnapshot, error) {
	return getSnapshot(ctx, ex, `SELECT `+userColumns+` FROM users u WHERE u.id = $1 FOR UPDATE`, id)
}

// SnapshotByEmailForUpdate đọc và khóa hàng user theo email chuẩn hóa tới hết transaction; không có → nil, nil.
func (PGUserRepo) SnapshotByEmailForUpdate(ctx context.Context, ex db.Executor, e domain.Email) (*UserSnapshot, error) {
	return getSnapshot(ctx, ex, `SELECT `+userColumns+` FROM users u WHERE u.email_normalized = $1 FOR UPDATE`, e.String())
}

// ActiveTeacher đọc giảng viên active và khóa FOR SHARE; không phải giảng viên active → nil, nil.
func (PGUserRepo) ActiveTeacher(ctx context.Context, ex db.Executor, id uuid.UUID) (*UserSnapshot, error) {
	return getSnapshot(ctx, ex, `SELECT `+userColumns+` FROM users u
		WHERE u.id = $1 AND u.role = 'teacher' AND u.status = 'active' FOR SHARE`, id)
}

// getSnapshot là getUser nhưng không tìm thấy → nil, nil.
func getSnapshot(ctx context.Context, ex db.Executor, q string, arg any) (*UserSnapshot, error) {
	u, err := getUser(ctx, ex, q, arg)
	if errors.Is(err, ErrUserNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	s := u.Snapshot()
	return &s, nil
}

func getUser(ctx context.Context, ex db.Executor, q string, arg any) (*User, error) {
	var r userRow
	if err := sqlx.GetContext(ctx, ex, &r, q, arg); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("identity: đọc user: %w", err)
	}
	return r.toUser()
}

// PGSessionRepo là SessionRepo trên Postgres.
type PGSessionRepo struct{}

var _ SessionRepo = PGSessionRepo{}

// Create chèn phiên mới.
func (PGSessionRepo) Create(ctx context.Context, ex db.Executor, s *Session) error {
	_, err := ex.ExecContext(ctx, `
		INSERT INTO sessions (id, user_id, token_hash, created_at, expires_at, last_seen_at, ip, user_agent)
		VALUES ($1, $2, $3, $4, $5, $6, $7::inet, $8)`,
		s.ID, s.UserID, s.TokenHash, s.CreatedAt, s.ExpiresAt, s.LastSeenAt, nullIfEmpty(s.IP), nullIfEmpty(s.UserAgent))
	if err != nil {
		return pgerr.Map(fmt.Errorf("identity: tạo phiên: %w", err))
	}
	return nil
}

// ByTokenHash đọc phiên chưa thu hồi cùng user chưa bị vô hiệu hóa.
func (PGSessionRepo) ByTokenHash(ctx context.Context, ex db.Executor, hash []byte) (*Session, *User, error) {
	var r struct {
		userRow
		SessionID  uuid.UUID `db:"session_id"`
		SCreatedAt time.Time `db:"s_created_at"`
		ExpiresAt  time.Time `db:"expires_at"`
		LastSeenAt time.Time `db:"last_seen_at"`
	}
	err := sqlx.GetContext(ctx, ex, &r, `
		SELECT `+userColumns+`, s.id AS session_id, s.created_at AS s_created_at, s.expires_at, s.last_seen_at
		FROM sessions s JOIN users u ON u.id = s.user_id
		WHERE s.token_hash = $1 AND s.revoked_at IS NULL AND u.status <> 'disabled'`, hash)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil, ErrSessionNotFound
		}
		return nil, nil, fmt.Errorf("identity: đọc phiên: %w", err)
	}
	u, err := r.toUser()
	if err != nil {
		return nil, nil, err
	}
	s := &Session{ID: r.SessionID, UserID: r.ID, TokenHash: hash, CreatedAt: r.SCreatedAt, ExpiresAt: r.ExpiresAt, LastSeenAt: r.LastSeenAt}
	return s, u, nil
}

// TouchLastSeen gia hạn phiên và ghi last_active_at của user, tối đa một lần mỗi phút cho mỗi phiên.
func (PGSessionRepo) TouchLastSeen(ctx context.Context, ex db.Executor, id uuid.UUID, now time.Time, ttl time.Duration) error {
	_, err := ex.ExecContext(ctx, `
		WITH s AS (
		  UPDATE sessions SET last_seen_at = $2, expires_at = $3
		  WHERE id = $1 AND last_seen_at < $4
		  RETURNING user_id
		)
		UPDATE users SET last_active_at = $2 FROM s WHERE users.id = s.user_id`,
		id, now, now.Add(ttl), now.Add(-touchEvery))
	if err != nil {
		return fmt.Errorf("identity: gia hạn phiên: %w", err)
	}
	return nil
}

// DeleteByID xóa phiên theo id (không có thì bỏ qua).
func (PGSessionRepo) DeleteByID(ctx context.Context, ex db.Executor, id uuid.UUID) error {
	if _, err := ex.ExecContext(ctx, `DELETE FROM sessions WHERE id = $1`, id); err != nil {
		return fmt.Errorf("identity: xóa phiên: %w", err)
	}
	return nil
}

// DeleteByTokenHash xóa phiên theo hash token (không có thì bỏ qua).
func (PGSessionRepo) DeleteByTokenHash(ctx context.Context, ex db.Executor, hash []byte) error {
	if _, err := ex.ExecContext(ctx, `DELETE FROM sessions WHERE token_hash = $1`, hash); err != nil {
		return fmt.Errorf("identity: xóa phiên: %w", err)
	}
	return nil
}

// DeleteAllForUser xóa mọi phiên của user trừ exceptID.
func (PGSessionRepo) DeleteAllForUser(ctx context.Context, ex db.Executor, userID uuid.UUID, exceptID *uuid.UUID) error {
	if _, err := ex.ExecContext(ctx, `DELETE FROM sessions WHERE user_id = $1 AND ($2::uuid IS NULL OR id <> $2)`,
		userID, exceptID); err != nil {
		return fmt.Errorf("identity: xóa phiên của user: %w", err)
	}
	return nil
}

// DeleteExpired xóa phiên đã hết hạn.
func (PGSessionRepo) DeleteExpired(ctx context.Context, ex db.Executor, now time.Time) (int64, error) {
	res, err := ex.ExecContext(ctx, `DELETE FROM sessions WHERE expires_at <= $1`, now)
	if err != nil {
		return 0, fmt.Errorf("identity: dọn phiên hết hạn: %w", err)
	}
	return res.RowsAffected()
}

// PGLoginAttemptRepo là LoginAttemptRepo trên Postgres.
type PGLoginAttemptRepo struct{}

var _ LoginAttemptRepo = PGLoginAttemptRepo{}

// LockEmail lấy advisory lock theo email trong transaction hiện tại.
func (PGLoginAttemptRepo) LockEmail(ctx context.Context, ex db.Executor, email domain.Email) error {
	if _, err := ex.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtext($1))`, email.String()); err != nil {
		return fmt.Errorf("identity: khóa email: %w", err)
	}
	return nil
}

// Record ghi một lần đăng nhập.
func (PGLoginAttemptRepo) Record(ctx context.Context, ex db.Executor, a LoginAttempt) error {
	if _, err := ex.ExecContext(ctx, `
		INSERT INTO login_attempts (email_normalized, ip, succeeded, attempted_at) VALUES ($1, $2::inet, $3, $4)`,
		a.EmailNormalized, nullIfEmpty(a.IP), a.Succeeded, a.AttemptedAt); err != nil {
		return fmt.Errorf("identity: ghi lần đăng nhập: %w", err)
	}
	return nil
}

// CountFailures đếm lần sai của email từ since và trả lần sai cũ nhất trong khoảng.
func (PGLoginAttemptRepo) CountFailures(ctx context.Context, ex db.Executor, email domain.Email, since time.Time) (int, *time.Time, error) {
	var r struct {
		N      int        `db:"n"`
		Oldest *time.Time `db:"oldest"`
	}
	if err := sqlx.GetContext(ctx, ex, &r, `
		SELECT count(*) AS n, min(attempted_at) AS oldest FROM login_attempts
		WHERE email_normalized = $1 AND NOT succeeded AND attempted_at > $2`, email.String(), since); err != nil {
		return 0, nil, fmt.Errorf("identity: đếm lần sai: %w", err)
	}
	return r.N, r.Oldest, nil
}

// DeleteOlderThan xóa bản ghi đăng nhập trước cutoff.
func (PGLoginAttemptRepo) DeleteOlderThan(ctx context.Context, ex db.Executor, cutoff time.Time) (int64, error) {
	res, err := ex.ExecContext(ctx, `DELETE FROM login_attempts WHERE attempted_at < $1`, cutoff)
	if err != nil {
		return 0, fmt.Errorf("identity: dọn login_attempts: %w", err)
	}
	return res.RowsAffected()
}

// PGResetTokenRepo là ResetTokenRepo trên Postgres.
type PGResetTokenRepo struct{}

var _ ResetTokenRepo = PGResetTokenRepo{}

// Create chèn token đặt lại.
func (PGResetTokenRepo) Create(ctx context.Context, ex db.Executor, t *PasswordResetToken) error {
	if _, err := ex.ExecContext(ctx, `
		INSERT INTO password_reset_tokens (id, user_id, token_hash, expires_at, created_at) VALUES ($1, $2, $3, $4, $5)`,
		t.ID, t.UserID, t.TokenHash, t.ExpiresAt, t.CreatedAt); err != nil {
		return pgerr.Map(fmt.Errorf("identity: tạo token đặt lại: %w", err))
	}
	return nil
}

// Consume tiêu thụ token bằng một câu UPDATE có điều kiện nên hai request đồng thời chỉ một bên thành công.
// Token của tài khoản đã bị vô hiệu hóa (vô hiệu hóa cũng đóng token) trả ErrAccountDisabled thay vì
// ErrResetTokenInvalid để người dùng biết cần liên hệ quản trị viên.
func (PGResetTokenRepo) Consume(ctx context.Context, ex db.Executor, hash []byte, now time.Time) (uuid.UUID, error) {
	var userID uuid.UUID
	err := sqlx.GetContext(ctx, ex, &userID, `
		UPDATE password_reset_tokens SET used_at = $2
		WHERE token_hash = $1 AND used_at IS NULL AND expires_at > $2
		RETURNING user_id`, hash, now)
	if err == nil {
		return userID, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return uuid.Nil, fmt.Errorf("identity: tiêu thụ token đặt lại: %w", err)
	}
	var status string
	err = sqlx.GetContext(ctx, ex, &status, `
		SELECT u.status FROM password_reset_tokens t JOIN users u ON u.id = t.user_id WHERE t.token_hash = $1`, hash)
	switch {
	case err == nil && status == string(domain.UserDisabled):
		return uuid.Nil, ErrAccountDisabled
	case err == nil || errors.Is(err, sql.ErrNoRows):
		return uuid.Nil, ErrResetTokenInvalid
	default:
		return uuid.Nil, fmt.Errorf("identity: đọc token đặt lại: %w", err)
	}
}

// InvalidateForUser đánh dấu đã dùng mọi token còn mở của user.
func (PGResetTokenRepo) InvalidateForUser(ctx context.Context, ex db.Executor, userID uuid.UUID, now time.Time) error {
	if _, err := ex.ExecContext(ctx, `
		UPDATE password_reset_tokens SET used_at = $2 WHERE user_id = $1 AND used_at IS NULL`, userID, now); err != nil {
		return fmt.Errorf("identity: vô hiệu token đặt lại: %w", err)
	}
	return nil
}

func nullIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
