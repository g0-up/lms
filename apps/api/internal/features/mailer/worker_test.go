package mailer

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"lms/api/internal/platform/clock"
	"lms/api/internal/platform/db"
	"lms/api/internal/platform/secretbox"
)

// directTx chạy fn ngay, không có transaction thật.
type directTx struct{}

func (directTx) Transact(ctx context.Context, fn func(db.Executor) error) error { return fn(nil) }

// recordSender ghi lại email đã gửi; err != nil thì mọi lần gửi đều lỗi.
type recordSender struct {
	mu   sync.Mutex
	sent []RenderedMail
	err  error
}

func (s *recordSender) Send(_ context.Context, m RenderedMail) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.err != nil {
		return s.err
	}
	s.sent = append(s.sent, m)
	return nil
}

type countCleanup struct {
	mu    sync.Mutex
	calls int
}

func (c *countCleanup) Cleanup(context.Context, time.Time) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.calls++
	return errors.New("dọn dẹp lỗi không được dừng worker")
}

func newTestWorker(t *testing.T, repo *memOutbox, sender Sender, box *secretbox.Box) (*Worker, *clock.Fake) {
	t.Helper()
	clk := &clock.Fake{T: entityNow}
	w, err := NewWorker(WorkerConfig{
		Tx: directTx{}, Repo: repo, Renderer: testRenderer(t), Sender: sender, Box: box, Clock: clk,
		PublicBaseURL: "http://localhost:5173",
	})
	require.NoError(t, err)
	return w, clk
}

func enqueue(t *testing.T, repo *memOutbox, box *secretbox.Box, msg Message) uuid.UUID {
	t.Helper()
	id, err := NewService(repo, box, &clock.Fake{T: entityNow}).Enqueue(context.Background(), nil, msg)
	require.NoError(t, err)
	return id
}

func TestWorkerSendsResetLinkAndClearsSecret(t *testing.T) {
	repo, box, sender := &memOutbox{}, testBox(t), &recordSender{}
	w, _ := newTestWorker(t, repo, sender, box)
	id := enqueue(t, repo, box, Message{
		To: mustEmail(t, "huong.le@goup.vn"), Template: TemplatePasswordReset,
		Payload: map[string]any{"Name": "Lê Hương", "ExpiresMinutes": 30}, Secret: "tok_ABC-123",
	})

	n, err := w.RunOnce(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 1, n)
	require.Len(t, sender.sent, 1)
	assert.Contains(t, sender.sent[0].Text, "http://localhost:5173/reset-password#token=tok_ABC-123")
	row := repo.find(id)
	assert.Equal(t, StatusSent, row.Status)
	assert.Nil(t, row.SecretEnc)

	n, err = w.RunOnce(context.Background())
	require.NoError(t, err)
	assert.Zero(t, n, "hàng đã gửi không được claim lại")
}

func TestWorkerInjectsTempPassword(t *testing.T) {
	repo, box, sender := &memOutbox{}, testBox(t), &recordSender{}
	w, _ := newTestWorker(t, repo, sender, box)
	enqueue(t, repo, box, Message{
		To: mustEmail(t, "minh.bui@gmail.com"), Template: TemplateInvite, Payload: classPayload(), Secret: "Ab3dEf7hJk9m",
	})
	_, err := w.RunOnce(context.Background())
	require.NoError(t, err)
	require.Len(t, sender.sent, 1)
	assert.Contains(t, sender.sent[0].HTML, "Ab3dEf7hJk9m")
}

func TestWorkerRecordsSendFailureWithBackoff(t *testing.T) {
	repo, box := &memOutbox{}, testBox(t)
	w, _ := newTestWorker(t, repo, &recordSender{err: errors.New("smtp: 421 thử lại sau")}, box)
	id := enqueue(t, repo, box, Message{To: mustEmail(t, "an.nguyen@gmail.com"), Template: TemplateAdded, Payload: classPayload()})

	n, err := w.RunOnce(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 1, n)
	row := repo.find(id)
	assert.Equal(t, StatusQueued, row.Status)
	assert.Equal(t, 1, row.Attempts)
	assert.Equal(t, "smtp: 421 thử lại sau", *row.LastError)
	assert.Equal(t, entityNow.Add(time.Minute), row.RunAt)
}

func TestWorkerFailsRowsWhoseSecretCannotBeOpened(t *testing.T) {
	repo, box := &memOutbox{}, testBox(t)
	otherKey, err := NewSecretBox("ZmVkY2JhOTg3NjU0MzIxMGZlZGNiYTk4NzY1NDMyMTA=")
	require.NoError(t, err)
	sender := &recordSender{}
	w, _ := newTestWorker(t, repo, sender, otherKey)
	wrongKey := enqueue(t, repo, box, Message{To: mustEmail(t, "a@example.com"), Template: TemplateInvite, Payload: classPayload(), Secret: "x"})
	noBox, _ := newTestWorker(t, repo, sender, nil)

	_, err = w.RunOnce(context.Background())
	require.NoError(t, err)
	assert.Equal(t, errSecretOpen, *repo.find(wrongKey).LastError)

	repo.find(wrongKey).Status = StatusQueued
	_, err = noBox.RunOnce(context.Background())
	require.NoError(t, err)
	assert.Equal(t, errSecretOpen, *repo.find(wrongKey).LastError)
	assert.Empty(t, sender.sent)
}

func TestWorkerFailsSecretTemplatesWithoutSecret(t *testing.T) {
	repo := &memOutbox{}
	sender := &recordSender{}
	w, _ := newTestWorker(t, repo, sender, testBox(t))
	// Hàng hỏng chèn thẳng (không qua Enqueue): invite mà không có secret_enc.
	id := uuid.New()
	repo.rows = append(repo.rows, &OutboxMessage{
		ID: id, ToEmail: "a@example.com", Template: TemplateInvite, Payload: classPayload(), Status: StatusQueued, RunAt: entityNow,
	})
	_, err := w.RunOnce(context.Background())
	require.NoError(t, err)
	assert.Contains(t, *repo.find(id).LastError, "thiếu secret_enc")
	assert.Empty(t, sender.sent)
}

func TestWorkerPropagatesClaimError(t *testing.T) {
	repo := &memOutbox{claimErr: errors.New("db down")}
	w, _ := newTestWorker(t, repo, &recordSender{}, nil)
	_, err := w.RunOnce(context.Background())
	require.Error(t, err)
}

func TestWorkerRunCleansUpAndStopsOnCancel(t *testing.T) {
	repo := &memOutbox{}
	cleanup := &countCleanup{}
	w, err := NewWorker(WorkerConfig{
		Tx: directTx{}, Repo: repo, Renderer: testRenderer(t), Sender: &recordSender{}, Clock: &clock.Fake{T: entityNow},
		PollInterval: time.Millisecond, Cleanups: []Cleanup{cleanup},
	})
	require.NoError(t, err)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	require.NoError(t, w.Run(ctx))
	cleanup.mu.Lock()
	defer cleanup.mu.Unlock()
	assert.Equal(t, 1, cleanup.calls, "đồng hồ đứng yên nên dọn dẹp chỉ chạy lần đầu")
}

func TestNewWorkerRequiresDependencies(t *testing.T) {
	_, err := NewWorker(WorkerConfig{})
	require.Error(t, err)
	w, err := NewWorker(WorkerConfig{
		Tx: directTx{}, Repo: &memOutbox{}, Renderer: testRenderer(t), Sender: &recordSender{}, Clock: clock.Real{},
	})
	require.NoError(t, err)
	assert.Equal(t, 5*time.Second, w.cfg.PollInterval)
}
