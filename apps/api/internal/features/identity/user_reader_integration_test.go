//go:build integration

package identity

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"lms/api/internal/domain"
	"lms/api/internal/platform/db"
	"lms/api/internal/platform/testdb"
)

func TestUserReaderSnapshots(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	r := PGUserRepo{}
	anID := uuid.MustParse(testdb.StudentAnNguyenID)

	s, err := r.SnapshotByID(ctx, h.dbx, anID)
	require.NoError(t, err)
	require.NotNil(t, s)
	assert.Equal(t, "an.nguyen@gmail.com", s.Email.String())
	assert.Equal(t, "Nguyễn Hoàng An", s.Name)
	assert.Equal(t, domain.RoleStudent, s.Role)
	assert.Equal(t, domain.UserActive, s.Status)
	assert.NotNil(t, s.LastLoginAt)

	// Tra theo email đã chuẩn hóa nên chữ hoa/khoảng trắng ở đầu vào không ảnh hưởng.
	byEmail, err := r.SnapshotByEmail(ctx, h.dbx, mustEmail(t, "  AN.Nguyen@Gmail.com "))
	require.NoError(t, err)
	require.NotNil(t, byEmail)
	assert.Equal(t, anID, byEmail.ID)

	missing, err := r.SnapshotByID(ctx, h.dbx, uuid.New())
	require.NoError(t, err)
	assert.Nil(t, missing)
	missing, err = r.SnapshotByEmail(ctx, h.dbx, mustEmail(t, "khong.co@gmail.com"))
	require.NoError(t, err)
	assert.Nil(t, missing)

	require.NoError(t, db.Transact(ctx, h.dbx, func(tx db.Executor) error {
		locked, err := r.SnapshotByIDForUpdate(ctx, tx, anID)
		require.NoError(t, err)
		require.NotNil(t, locked)
		lockedByEmail, err := r.SnapshotByEmailForUpdate(ctx, tx, mustEmail(t, "an.nguyen@gmail.com"))
		require.NoError(t, err)
		require.NotNil(t, lockedByEmail)
		assert.Equal(t, anID, lockedByEmail.ID)
		none, err := r.SnapshotByEmailForUpdate(ctx, tx, mustEmail(t, "khong.co@gmail.com"))
		require.NoError(t, err)
		assert.Nil(t, none)
		return nil
	}))
}

func TestUserReaderSnapshotByIDs(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	r := PGUserRepo{}
	anID := uuid.MustParse(testdb.StudentAnNguyenID)
	huongID := uuid.MustParse(testdb.TeacherHuongLeID)

	got, err := r.SnapshotByIDs(ctx, h.dbx, []uuid.UUID{anID, huongID, uuid.New(), anID})
	require.NoError(t, err)
	require.Len(t, got, 2)
	assert.Equal(t, "Nguyễn Hoàng An", got[anID].Name)
	assert.Equal(t, "huong.le@goup.vn", got[huongID].Email.String())

	empty, err := r.SnapshotByIDs(ctx, h.dbx, nil)
	require.NoError(t, err)
	assert.Empty(t, empty)
}

func TestUserReaderActiveTeacher(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	r := PGUserRepo{}
	huongID := uuid.MustParse(testdb.TeacherHuongLeID)

	s, err := r.ActiveTeacher(ctx, h.dbx, huongID)
	require.NoError(t, err)
	require.NotNil(t, s)
	assert.Equal(t, "Lê Thu Hương", s.Name)

	for _, id := range []string{testdb.AdminQuanTranID, testdb.StudentAnNguyenID, uuid.NewString()} {
		s, err := r.ActiveTeacher(ctx, h.dbx, uuid.MustParse(id))
		require.NoError(t, err)
		assert.Nil(t, s, "không phải giảng viên: %s", id)
	}

	_, err = h.dbx.ExecContext(ctx, `UPDATE users SET status = 'disabled', disabled_at = now() WHERE id = $1`, huongID)
	require.NoError(t, err)
	s, err = r.ActiveTeacher(ctx, h.dbx, huongID)
	require.NoError(t, err)
	assert.Nil(t, s, "giảng viên bị vô hiệu hóa")
}

// TestUserReaderLocksBlockConcurrentWrites xác nhận khóa giữ tới hết tx: lần ghi song song lên cùng user phải chờ.
func TestUserReaderLocksBlockConcurrentWrites(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	r := PGUserRepo{}

	tests := []struct {
		name string
		id   string
		read func(ex db.Executor, id uuid.UUID) (*UserSnapshot, error)
	}{
		{"FOR UPDATE theo id", testdb.StudentAnNguyenID, func(ex db.Executor, id uuid.UUID) (*UserSnapshot, error) {
			return r.SnapshotByIDForUpdate(ctx, ex, id)
		}},
		{"FOR UPDATE theo email", testdb.StudentAnNguyenID, func(ex db.Executor, _ uuid.UUID) (*UserSnapshot, error) {
			return r.SnapshotByEmailForUpdate(ctx, ex, mustEmail(t, "an.nguyen@gmail.com"))
		}},
		{"FOR SHARE giảng viên", testdb.TeacherHuongLeID, func(ex db.Executor, id uuid.UUID) (*UserSnapshot, error) {
			return r.ActiveTeacher(ctx, ex, id)
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id := uuid.MustParse(tt.id)
			require.NoError(t, db.Transact(ctx, h.dbx, func(tx db.Executor) error {
				s, err := tt.read(tx, id)
				require.NoError(t, err)
				require.NotNil(t, s)
				assert.ErrorContains(t, updateWithLockTimeout(ctx, h.dbx, id), "lock timeout")
				return nil
			}))
			// Hết tx thì khóa được nhả.
			require.NoError(t, updateWithLockTimeout(ctx, h.dbx, id))
		})
	}
}

func updateWithLockTimeout(ctx context.Context, dbx *sqlx.DB, id uuid.UUID) error {
	return db.Transact(ctx, dbx, func(tx db.Executor) error {
		if _, err := tx.ExecContext(ctx, `SET LOCAL lock_timeout = '200ms'`); err != nil {
			return err
		}
		_, err := tx.ExecContext(ctx, `UPDATE users SET full_name = full_name WHERE id = $1`, id)
		return err
	})
}
