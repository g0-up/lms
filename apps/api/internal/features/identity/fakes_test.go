package identity

import (
	"bytes"
	"context"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"lms/api/internal/domain"
	"lms/api/internal/features/mailer"
	"lms/api/internal/platform/audit"
	"lms/api/internal/platform/clock"
	"lms/api/internal/platform/config"
	"lms/api/internal/platform/db"
)

// memStore là bộ repository trong bộ nhớ cho unit test service/middleware. fakeTx không rollback nên test nhánh
// cần rollback nằm ở service_integration_test.go.
type memStore struct {
	mu       sync.Mutex
	users    map[uuid.UUID]UserSnapshot
	hashes   map[uuid.UUID]PasswordHash
	sessions map[uuid.UUID]Session
	attempts []LoginAttempt
	resets   []PasswordResetToken
	mails    []mailer.Message
	audits   []audit.Entry
}

func newMemStore() *memStore {
	return &memStore{
		users: map[uuid.UUID]UserSnapshot{}, hashes: map[uuid.UUID]PasswordHash{}, sessions: map[uuid.UUID]Session{},
	}
}

type fakeTx struct{}

func (fakeTx) Transact(_ context.Context, fn func(db.Executor) error) error { return fn(nil) }

func (m *memStore) put(u *User) {
	m.users[u.ID()] = u.Snapshot()
	m.hashes[u.ID()] = u.PasswordHash()
}

func (m *memStore) get(id uuid.UUID) (*User, error) {
	s, ok := m.users[id]
	if !ok {
		return nil, ErrUserNotFound
	}
	return RehydrateUser(s, m.hashes[id]), nil
}

type memUsers struct{ *memStore }

func (r memUsers) Create(_ context.Context, _ db.Executor, u *User) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, s := range r.users {
		if s.Email == u.Email() {
			return ErrEmailTaken
		}
	}
	r.put(u)
	return nil
}

func (r memUsers) Update(_ context.Context, _ db.Executor, u *User) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.users[u.ID()]; !ok {
		return ErrUserNotFound
	}
	r.put(u)
	return nil
}

func (r memUsers) ByID(_ context.Context, _ db.Executor, id uuid.UUID) (*User, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.get(id)
}

func (r memUsers) ByIDForUpdate(ctx context.Context, ex db.Executor, id uuid.UUID) (*User, error) {
	return r.ByID(ctx, ex, id)
}

func (r memUsers) ByEmail(_ context.Context, _ db.Executor, e domain.Email) (*User, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for id, s := range r.users {
		if s.Email.String() == e.String() {
			return r.get(id)
		}
	}
	return nil, ErrUserNotFound
}

func (r memUsers) ListByRole(_ context.Context, _ db.Executor, role domain.Role, status *domain.UserStatus) ([]*User, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []*User
	for id, s := range r.users {
		if s.Role == role && (status == nil || s.Status == *status) {
			u, _ := r.get(id)
			out = append(out, u)
		}
	}
	return out, nil
}

type memSessions struct{ *memStore }

func (r memSessions) Create(_ context.Context, _ db.Executor, s *Session) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.sessions[s.ID] = *s
	return nil
}

func (r memSessions) ByTokenHash(_ context.Context, _ db.Executor, hash []byte) (*Session, *User, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, s := range r.sessions {
		if bytes.Equal(s.TokenHash, hash) {
			u, err := r.get(s.UserID)
			if err != nil || u.Status() == domain.UserDisabled {
				return nil, nil, ErrSessionNotFound
			}
			return &s, u, nil
		}
	}
	return nil, nil, ErrSessionNotFound
}

func (r memSessions) TouchLastSeen(_ context.Context, _ db.Executor, id uuid.UUID, now time.Time, ttl time.Duration) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	s, ok := r.sessions[id]
	if !ok || now.Sub(s.LastSeenAt) <= touchEvery {
		return nil
	}
	s.LastSeenAt, s.ExpiresAt = now, now.Add(ttl)
	r.sessions[id] = s
	return nil
}

func (r memSessions) DeleteByID(_ context.Context, _ db.Executor, id uuid.UUID) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.sessions, id)
	return nil
}

func (r memSessions) DeleteByTokenHash(_ context.Context, _ db.Executor, hash []byte) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for id, s := range r.sessions {
		if bytes.Equal(s.TokenHash, hash) {
			delete(r.sessions, id)
		}
	}
	return nil
}

func (r memSessions) DeleteAllForUser(_ context.Context, _ db.Executor, userID uuid.UUID, exceptID *uuid.UUID) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for id, s := range r.sessions {
		if s.UserID == userID && (exceptID == nil || id != *exceptID) {
			delete(r.sessions, id)
		}
	}
	return nil
}

func (r memSessions) DeleteExpired(_ context.Context, _ db.Executor, now time.Time) (int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var n int64
	for id, s := range r.sessions {
		if s.IsExpired(now) {
			delete(r.sessions, id)
			n++
		}
	}
	return n, nil
}

type memAttempts struct{ *memStore }

func (memAttempts) LockEmail(context.Context, db.Executor, domain.Email) error { return nil }

func (r memAttempts) Record(_ context.Context, _ db.Executor, a LoginAttempt) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.attempts = append(r.attempts, a)
	return nil
}

func (r memAttempts) CountFailures(_ context.Context, _ db.Executor, email domain.Email, since time.Time) (int, *time.Time, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var (
		n      int
		oldest *time.Time
	)
	for _, a := range r.attempts {
		if a.EmailNormalized == email.String() && !a.Succeeded && a.AttemptedAt.After(since) {
			n++
			if oldest == nil || a.AttemptedAt.Before(*oldest) {
				at := a.AttemptedAt
				oldest = &at
			}
		}
	}
	return n, oldest, nil
}

func (r memAttempts) DeleteOlderThan(_ context.Context, _ db.Executor, cutoff time.Time) (int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	before := len(r.attempts)
	r.attempts = slices.DeleteFunc(r.attempts, func(a LoginAttempt) bool { return a.AttemptedAt.Before(cutoff) })
	return int64(before - len(r.attempts)), nil
}

type memResets struct{ *memStore }

func (r memResets) Create(_ context.Context, _ db.Executor, t *PasswordResetToken) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.resets = append(r.resets, *t)
	return nil
}

func (r memResets) Consume(_ context.Context, _ db.Executor, hash []byte, now time.Time) (uuid.UUID, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for i, t := range r.resets {
		if bytes.Equal(t.TokenHash, hash) && t.UsedAt == nil && now.Before(t.ExpiresAt) {
			r.resets[i].UsedAt = &now
			return t.UserID, nil
		}
	}
	return uuid.Nil, ErrResetTokenInvalid
}

func (r memResets) InvalidateForUser(_ context.Context, _ db.Executor, userID uuid.UUID, now time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for i, t := range r.resets {
		if t.UserID == userID && t.UsedAt == nil {
			r.resets[i].UsedAt = &now
		}
	}
	return nil
}

type memMailer struct{ *memStore }

func (r memMailer) Enqueue(_ context.Context, _ db.Executor, msg mailer.Message) (uuid.UUID, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.mails = append(r.mails, msg)
	return uuid.New(), nil
}

type memAudit struct{ *memStore }

func (r memAudit) Record(_ context.Context, _ db.Executor, e audit.Entry) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.audits = append(r.audits, e)
	return nil
}

func testServiceConfig() config.Config {
	return config.Config{
		SessionTTL: 12 * time.Hour, TempPasswordTTL: 72 * time.Hour, ResetTokenTTL: 30 * time.Minute,
		PasswordMinLength: 8, LoginMaxFailures: 5, LoginLockWindow: 15 * time.Minute,
		PublicBaseURL: "http://localhost:5173",
	}
}

// newMemService dựng Service trên memStore với đồng hồ giả.
func newMemService(t *testing.T) (*Service, *memStore, *clock.Fake) {
	t.Helper()
	m := newMemStore()
	clk := &clock.Fake{T: time.Date(2026, 10, 5, 9, 0, 0, 0, time.UTC)}
	svc := NewService(ServiceDeps{
		Tx: fakeTx{}, Users: memUsers{m}, Sessions: memSessions{m}, Attempts: memAttempts{m}, Resets: memResets{m},
		Mailer: memMailer{m}, Clock: clk, Audit: memAudit{m}, Cfg: testServiceConfig(),
	})
	return svc, m, clk
}

// addUser lưu user với mật khẩu plain (hash argon2 thật để service verify được).
func addUser(t *testing.T, m *memStore, u func(PasswordHash) (*User, error), plain string) *User {
	t.Helper()
	h, err := NewPasswordHash(plain)
	require.NoError(t, err)
	user, err := u(h)
	require.NoError(t, err)
	m.mu.Lock()
	m.put(user)
	m.mu.Unlock()
	return user
}
