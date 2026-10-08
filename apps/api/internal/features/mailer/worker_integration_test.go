//go:build integration

package mailer

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"lms/api/internal/platform/clock"
	"lms/api/internal/platform/db"
	"lms/api/internal/platform/testdb"
)

// pgHarness dựng outbox trên Postgres thật. Đồng hồ giả bắt đầu ở giờ thật vì câu claim so run_at với now().
type pgHarness struct {
	dbx  *sqlx.DB
	clk  *clock.Fake
	repo PGOutboxRepo
	svc  *Service
}

func newPGHarness(t *testing.T) *pgHarness {
	t.Helper()
	dbx := testdb.Open(t)
	testdb.Reset(t, dbx)
	clk := &clock.Fake{T: time.Now().UTC().Truncate(time.Microsecond)}
	return &pgHarness{dbx: dbx, clk: clk, repo: NewOutboxRepo(), svc: NewService(NewOutboxRepo(), testBox(t), clk)}
}

func (h *pgHarness) worker(t *testing.T, sender Sender) *Worker {
	t.Helper()
	w, err := NewWorker(WorkerConfig{
		Tx: db.TxRunner{DB: h.dbx}, Repo: h.repo, Renderer: testRenderer(t), Sender: sender, Box: testBox(t),
		Clock: h.clk, PublicBaseURL: "http://localhost:5173",
	})
	require.NoError(t, err)
	return w
}

func (h *pgHarness) enqueue(t *testing.T, msg Message) uuid.UUID {
	t.Helper()
	id, err := h.svc.Enqueue(context.Background(), h.dbx, msg)
	require.NoError(t, err)
	return id
}

type outboxState struct {
	Status    string     `db:"status"`
	Attempts  int        `db:"attempts"`
	RunAt     time.Time  `db:"run_at"`
	LastError *string    `db:"last_error"`
	SecretEnc []byte     `db:"secret_enc"`
	SentAt    *time.Time `db:"sent_at"`
	Payload   string     `db:"payload"`
}

func (h *pgHarness) state(t *testing.T, id uuid.UUID) outboxState {
	t.Helper()
	var s outboxState
	require.NoError(t, h.dbx.GetContext(context.Background(), &s, `
		SELECT status, attempts, run_at, last_error, secret_enc, sent_at, payload::text AS payload
		FROM email_outbox WHERE id = $1`, id))
	return s
}

// makeDue đưa run_at về quá khứ để lượt claim kế tiếp nhận hàng mà không phải chờ backoff thật.
func (h *pgHarness) makeDue(t *testing.T, id uuid.UUID) {
	t.Helper()
	_, err := h.dbx.ExecContext(context.Background(), `UPDATE email_outbox SET run_at = now() - interval '1 second' WHERE id = $1`, id)
	require.NoError(t, err)
}

func inviteMessage(t *testing.T, to, secret string) Message {
	return Message{To: mustEmail(t, to), Template: TemplateInvite, Payload: classPayload(), Secret: secret}
}

func TestWorkerSendClearsSecretAndKeepsPayloadClean(t *testing.T) {
	h := newPGHarness(t)
	sender := &recordSender{}
	id := h.enqueue(t, inviteMessage(t, "minh.bui@gmail.com", "Ab3dEf7hJk9m"))

	queued := h.state(t, id)
	assert.Equal(t, StatusQueued, queued.Status)
	assert.NotEmpty(t, queued.SecretEnc)
	assert.NotContains(t, queued.Payload, "Ab3dEf7hJk9m")
	assert.NotContains(t, string(queued.SecretEnc), "Ab3dEf7hJk9m")

	n, err := h.worker(t, sender).RunOnce(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 1, n)
	require.Len(t, sender.sent, 1)
	assert.Contains(t, sender.sent[0].Text, "Ab3dEf7hJk9m")

	sent := h.state(t, id)
	assert.Equal(t, StatusSent, sent.Status)
	assert.Nil(t, sent.SecretEnc)
	require.NotNil(t, sent.SentAt)
	assert.WithinDuration(t, h.clk.Now(), *sent.SentAt, time.Microsecond)
}

func TestWorkerRetriesWithBackoffThenFails(t *testing.T) {
	h := newPGHarness(t)
	w := h.worker(t, &recordSender{err: errors.New("smtp: 421 máy chủ bận")})
	id := h.enqueue(t, inviteMessage(t, "minh.bui@gmail.com", "Ab3dEf7hJk9m"))
	ctx := context.Background()

	for i, backoff := range []time.Duration{time.Minute, 5 * time.Minute} {
		n, err := w.RunOnce(ctx)
		require.NoError(t, err)
		require.Equal(t, 1, n, "lần %d", i+1)
		s := h.state(t, id)
		assert.Equal(t, StatusQueued, s.Status)
		assert.Equal(t, i+1, s.Attempts)
		assert.WithinDuration(t, h.clk.Now().Add(backoff), s.RunAt, time.Microsecond)
		assert.Equal(t, "smtp: 421 máy chủ bận", *s.LastError)
		assert.NotEmpty(t, s.SecretEnc, "còn lượt gửi thì giữ bí mật")

		n, err = w.RunOnce(ctx)
		require.NoError(t, err)
		assert.Zero(t, n, "chưa tới run_at thì không claim")
		h.makeDue(t, id)
	}

	n, err := w.RunOnce(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, n)
	s := h.state(t, id)
	assert.Equal(t, StatusFailed, s.Status)
	assert.Equal(t, MaxAttempts, s.Attempts)
	assert.Equal(t, "smtp: 421 máy chủ bận", *s.LastError)
	assert.Nil(t, s.SecretEnc)

	h.makeDue(t, id)
	n, err = w.RunOnce(ctx)
	require.NoError(t, err)
	assert.Zero(t, n, "hàng failed không được claim lại")
}

func TestExpiredLeaseIsReclaimedWithoutCountingAttempt(t *testing.T) {
	h := newPGHarness(t)
	ctx := context.Background()
	id := h.enqueue(t, Message{To: mustEmail(t, "an.nguyen@gmail.com"), Template: TemplateAdded, Payload: classPayload()})

	claimed, err := h.repo.Claim(ctx, h.dbx, 10)
	require.NoError(t, err)
	require.Len(t, claimed, 1)
	again, err := h.repo.Claim(ctx, h.dbx, 10)
	require.NoError(t, err)
	assert.Empty(t, again, "lease 2 phút còn hiệu lực")

	_, err = h.dbx.ExecContext(ctx, `UPDATE email_outbox SET locked_until = now() - interval '1 second' WHERE id = $1`, id)
	require.NoError(t, err)
	again, err = h.repo.Claim(ctx, h.dbx, 10)
	require.NoError(t, err)
	require.Len(t, again, 1)
	assert.Equal(t, id, again[0].ID)
	assert.Zero(t, again[0].Attempts)
}

func TestSupersedeQueuedSkipsOldInvites(t *testing.T) {
	h := newPGHarness(t)
	ctx := context.Background()
	old1 := h.enqueue(t, inviteMessage(t, "minh.bui@gmail.com", "cu-1"))
	old2 := h.enqueue(t, Message{To: mustEmail(t, "minh.bui@gmail.com"), Template: TemplateResend, Payload: classPayload(), Secret: "cu-2"})
	added := h.enqueue(t, Message{To: mustEmail(t, "minh.bui@gmail.com"), Template: TemplateAdded, Payload: classPayload()})
	other := h.enqueue(t, inviteMessage(t, "an.nguyen@gmail.com", "khac"))

	n, err := h.repo.SupersedeQueued(ctx, h.dbx, "Minh.Bui@Gmail.com", []string{string(TemplateInvite), string(TemplateResend)}, h.clk.Now())
	require.NoError(t, err)
	assert.EqualValues(t, 2, n)
	for _, id := range []uuid.UUID{old1, old2} {
		s := h.state(t, id)
		assert.Equal(t, StatusFailed, s.Status)
		assert.Equal(t, "superseded", *s.LastError)
		assert.Nil(t, s.SecretEnc)
	}
	assert.Equal(t, StatusQueued, h.state(t, added).Status)
	assert.Equal(t, StatusQueued, h.state(t, other).Status)

	_, err = h.repo.SupersedeQueued(ctx, h.dbx, "minh.bui@gmail.com", nil, h.clk.Now())
	require.Error(t, err)

	sender := &recordSender{}
	_, err = h.worker(t, sender).RunOnce(ctx)
	require.NoError(t, err)
	assert.Len(t, sender.sent, 2, "chỉ gửi added và lời mời của học viên khác")
}

func TestClaimSkipsRowsLockedByAnotherTransaction(t *testing.T) {
	h := newPGHarness(t)
	ctx := context.Background()
	for i := range 6 {
		h.enqueue(t, Message{To: mustEmail(t, fmt.Sprintf("hv%d@example.com", i)), Template: TemplateAdded, Payload: classPayload()})
	}
	tx1, err := h.dbx.BeginTxx(ctx, nil)
	require.NoError(t, err)
	defer func() { _ = tx1.Rollback() }()
	first, err := h.repo.Claim(ctx, tx1, 3)
	require.NoError(t, err)
	require.Len(t, first, 3)

	// tx1 chưa commit nên hàng của nó còn status queued với tx khác; SKIP LOCKED phải bỏ qua thay vì chờ.
	claimCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	second, err := h.repo.Claim(claimCtx, h.dbx, 10)
	require.NoError(t, err)
	require.Len(t, second, 3)
	seen := map[uuid.UUID]bool{}
	for _, m := range append(first, second...) {
		assert.False(t, seen[m.ID], "hai transaction claim trùng %s", m.ID)
		seen[m.ID] = true
	}
}

// slowSender giả lập SMTP chậm để hai worker thật sự chạy chồng lên nhau.
type slowSender struct{ recordSender }

func (s *slowSender) Send(ctx context.Context, m RenderedMail) error {
	time.Sleep(2 * time.Millisecond)
	return s.recordSender.Send(ctx, m)
}

func TestTwoWorkersNeverSendTheSameRow(t *testing.T) {
	h := newPGHarness(t)
	const total = 40
	for i := range total {
		h.enqueue(t, Message{To: mustEmail(t, fmt.Sprintf("hv%02d@example.com", i)), Template: TemplateAdded, Payload: classPayload()})
	}
	senders := []*slowSender{{}, {}}
	var wg sync.WaitGroup
	errs := make([]error, len(senders))
	for i, s := range senders {
		w := h.worker(t, s)
		wg.Go(func() {
			for {
				n, err := w.RunOnce(context.Background())
				if err != nil || n == 0 {
					errs[i] = err
					return
				}
			}
		})
	}
	wg.Wait()
	require.NoError(t, errors.Join(errs...))

	seen := map[string]int{}
	for _, s := range senders {
		for _, m := range s.sent {
			seen[m.To]++
		}
	}
	assert.Len(t, seen, total)
	for to, c := range seen {
		assert.Equal(t, 1, c, "gửi trùng %s", to)
	}
	var sent int
	require.NoError(t, h.dbx.GetContext(context.Background(), &sent, `SELECT count(*) FROM email_outbox WHERE status = 'sent'`))
	assert.Equal(t, total, sent)
}

func TestStatusByIDs(t *testing.T) {
	h := newPGHarness(t)
	ctx := context.Background()
	id := h.enqueue(t, Message{To: mustEmail(t, "an.nguyen@gmail.com"), Template: TemplateAdded, Payload: classPayload()})
	missing := uuid.New()

	got, err := h.repo.StatusByIDs(ctx, h.dbx, []uuid.UUID{id, missing})
	require.NoError(t, err)
	require.Contains(t, got, id)
	assert.NotContains(t, got, missing)
	assert.Equal(t, StatusQueued, got[id].Status)
	assert.Nil(t, got[id].SentAt)

	_, err = h.worker(t, &recordSender{}).RunOnce(ctx)
	require.NoError(t, err)
	got, err = h.repo.StatusByIDs(ctx, h.dbx, []uuid.UUID{id})
	require.NoError(t, err)
	assert.Equal(t, StatusSent, got[id].Status)
	assert.NotNil(t, got[id].SentAt)

	empty, err := h.repo.StatusByIDs(ctx, h.dbx, nil)
	require.NoError(t, err)
	assert.Empty(t, empty)
}

func TestLastErrorNeverContainsSecret(t *testing.T) {
	h := newPGHarness(t)
	// Lỗi template (thiếu khóa) chỉ nêu tên khóa: hàng resend có secret nhưng payload thiếu ClassName.
	payload := classPayload()
	delete(payload, "ClassName")
	id := h.enqueue(t, Message{To: mustEmail(t, "minh.bui@gmail.com"), Template: TemplateResend, Payload: payload, Secret: "Ab3dEf7hJk9m"})
	_, err := h.worker(t, &recordSender{}).RunOnce(context.Background())
	require.NoError(t, err)
	s := h.state(t, id)
	require.NotNil(t, s.LastError)
	assert.Contains(t, *s.LastError, "ClassName")
	assert.False(t, strings.Contains(*s.LastError, "Ab3dEf7hJk9m"))
}
