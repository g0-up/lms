package mailer

import (
	"context"
	"errors"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"lms/api/internal/domain"
	"lms/api/internal/platform/clock"
	"lms/api/internal/platform/db"
	"lms/api/internal/platform/secretbox"
)

// testKey là OUTBOX_SECRET_KEY chỉ dùng trong test (base64 của 32 byte).
const testKey = "MDEyMzQ1Njc4OWFiY2RlZjAxMjM0NTY3ODlhYmNkZWY="

func testBox(t *testing.T) *secretbox.Box {
	t.Helper()
	box, err := NewSecretBox(testKey)
	require.NoError(t, err)
	return box
}

// memOutbox là OutboxRepo trong bộ nhớ cho unit test service/worker.
type memOutbox struct {
	mu       sync.Mutex
	rows     []*OutboxMessage
	claimErr error
}

func (r *memOutbox) Insert(_ context.Context, _ db.Executor, m *OutboxMessage) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	cp := *m
	r.rows = append(r.rows, &cp)
	return nil
}

func (r *memOutbox) Claim(_ context.Context, _ db.Executor, limit int) ([]*OutboxMessage, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.claimErr != nil {
		return nil, r.claimErr
	}
	var out []*OutboxMessage
	for _, m := range r.rows {
		if m.Status == StatusQueued && len(out) < limit {
			m.Status = StatusSending
			cp := *m
			out = append(out, &cp)
		}
	}
	return out, nil
}

func (r *memOutbox) find(id uuid.UUID) *OutboxMessage {
	i := slices.IndexFunc(r.rows, func(m *OutboxMessage) bool { return m.ID == id })
	if i < 0 {
		return nil
	}
	return r.rows[i]
}

func (r *memOutbox) MarkSent(_ context.Context, _ db.Executor, id uuid.UUID, now time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.find(id).MarkSent(now)
	return nil
}

func (r *memOutbox) MarkFailedAttempt(_ context.Context, _ db.Executor, m *OutboxMessage) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	row := r.find(m.ID)
	*row = *m
	return nil
}

func (r *memOutbox) StatusByIDs(context.Context, db.Executor, []uuid.UUID) (map[uuid.UUID]DeliveryStatus, error) {
	return nil, errors.New("không dùng trong unit test")
}

func (r *memOutbox) SupersedeQueued(context.Context, db.Executor, string, []string, time.Time) (int64, error) {
	return 0, errors.New("không dùng trong unit test")
}

func mustEmail(t *testing.T, s string) domain.Email {
	t.Helper()
	e, err := domain.ParseEmail(s)
	require.NoError(t, err)
	return e
}

func TestEnqueueSealsSecretAndQueues(t *testing.T) {
	repo := &memOutbox{}
	box := testBox(t)
	clk := &clock.Fake{T: entityNow}
	svc := NewService(repo, box, clk)

	id, err := svc.Enqueue(context.Background(), nil, Message{
		To: mustEmail(t, "Minh.Bui@gmail.com"), Template: TemplateInvite,
		Payload: map[string]any{"Name": "Bùi Minh"}, Secret: "Ab3dEf7hJk9m",
	})
	require.NoError(t, err)
	require.Len(t, repo.rows, 1)
	row := repo.rows[0]
	assert.Equal(t, id, row.ID)
	assert.Equal(t, "Minh.Bui@gmail.com", row.ToEmail)
	assert.Equal(t, StatusQueued, row.Status)
	assert.Equal(t, entityNow, row.RunAt)
	assert.Zero(t, row.Attempts)
	assert.Equal(t, map[string]any{"Name": "Bùi Minh"}, row.Payload)
	assert.NotContains(t, string(row.SecretEnc), "Ab3dEf7hJk9m")
	plain, err := box.Open(row.SecretEnc)
	require.NoError(t, err)
	assert.Equal(t, "Ab3dEf7hJk9m", string(plain))
}

func TestEnqueueWithoutSecretStoresNullSecret(t *testing.T) {
	repo := &memOutbox{}
	svc := NewService(repo, nil, &clock.Fake{T: entityNow})
	_, err := svc.Enqueue(context.Background(), nil, Message{To: mustEmail(t, "an.nguyen@gmail.com"), Template: TemplateAdded})
	require.NoError(t, err)
	assert.Nil(t, repo.rows[0].SecretEnc)
	assert.NotNil(t, repo.rows[0].Payload, "payload jsonb NOT NULL nên nil thành {}")
}

func TestEnqueueRejectsInvalidMessages(t *testing.T) {
	repo := &memOutbox{}
	withBox := NewService(repo, testBox(t), &clock.Fake{T: entityNow})
	noBox := NewService(repo, nil, &clock.Fake{T: entityNow})
	to := mustEmail(t, "an.nguyen@gmail.com")
	ctx := context.Background()

	_, err := withBox.Enqueue(ctx, nil, Message{To: to, Template: "account_disabled"})
	require.ErrorIs(t, err, ErrUnknownTemplate)
	_, err = withBox.Enqueue(ctx, nil, Message{Template: TemplateAdded})
	require.Error(t, err)
	for _, k := range []string{"TempPassword", "Link", "Token"} {
		_, err = withBox.Enqueue(ctx, nil, Message{To: to, Template: TemplateInvite, Payload: map[string]any{k: "x"}})
		require.ErrorIs(t, err, ErrSecretInPayload, k)
	}
	_, err = noBox.Enqueue(ctx, nil, Message{To: to, Template: TemplateInvite, Secret: "x"})
	require.ErrorIs(t, err, ErrNoSecretBox)
	assert.Empty(t, repo.rows)
}

func TestNewSecretBoxValidatesKeyWithoutLeakingIt(t *testing.T) {
	_, err := NewSecretBox("khong-phai-base64!")
	require.ErrorIs(t, err, errSecretKey)
	assert.NotContains(t, err.Error(), "khong-phai-base64")
	_, err = NewSecretBox("c2hvcnQ=") // 5 byte
	require.ErrorIs(t, err, errSecretKey)
	_, err = NewSecretBox(testKey)
	require.NoError(t, err)
}
