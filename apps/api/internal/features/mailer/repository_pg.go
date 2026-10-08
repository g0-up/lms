package mailer

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jmoiron/sqlx"

	"lms/api/internal/platform/db"
	"lms/api/internal/platform/db/pgerr"
)

// PGOutboxRepo là OutboxRepo trên Postgres.
type PGOutboxRepo struct{}

var _ OutboxRepo = PGOutboxRepo{}

// NewOutboxRepo trả OutboxRepo Postgres.
func NewOutboxRepo() PGOutboxRepo { return PGOutboxRepo{} }

const outboxColumns = `id, to_email, template, payload, secret_enc, status, attempts, run_at, locked_until, last_error, sent_at, created_at`

type outboxRow struct {
	ID          uuid.UUID  `db:"id"`
	ToEmail     string     `db:"to_email"`
	Template    string     `db:"template"`
	Payload     []byte     `db:"payload"`
	SecretEnc   []byte     `db:"secret_enc"`
	Status      string     `db:"status"`
	Attempts    int        `db:"attempts"`
	RunAt       time.Time  `db:"run_at"`
	LockedUntil *time.Time `db:"locked_until"`
	LastError   *string    `db:"last_error"`
	SentAt      *time.Time `db:"sent_at"`
	CreatedAt   time.Time  `db:"created_at"`
}

func (r outboxRow) toMessage() (*OutboxMessage, error) {
	payload := map[string]any{}
	if len(r.Payload) > 0 {
		if err := json.Unmarshal(r.Payload, &payload); err != nil {
			return nil, fmt.Errorf("mailer: payload hàng %s: %w", r.ID, err)
		}
	}
	return &OutboxMessage{
		ID: r.ID, ToEmail: r.ToEmail, Template: TemplateName(r.Template), Payload: payload, SecretEnc: r.SecretEnc,
		Status: r.Status, Attempts: r.Attempts, RunAt: r.RunAt, LockedUntil: r.LockedUntil, LastError: r.LastError,
		SentAt: r.SentAt, CreatedAt: r.CreatedAt,
	}, nil
}

// Insert ghi hàng mới với trạng thái, run_at và secret_enc của m.
func (PGOutboxRepo) Insert(ctx context.Context, ex db.Executor, m *OutboxMessage) error {
	payload, err := json.Marshal(m.Payload)
	if err != nil {
		return fmt.Errorf("mailer: payload: %w", err)
	}
	_, err = ex.ExecContext(ctx, `
		INSERT INTO email_outbox (id, to_email, template, payload, secret_enc, status, attempts, run_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		m.ID, m.ToEmail, string(m.Template), string(payload), m.SecretEnc, m.Status, m.Attempts, m.RunAt)
	if err != nil {
		return pgerr.Map(fmt.Errorf("mailer: insert outbox: %w", err))
	}
	return nil
}

// Claim chạy câu claim với FOR UPDATE SKIP LOCKED nên nhiều worker song song không nhận trùng hàng.
func (PGOutboxRepo) Claim(ctx context.Context, ex db.Executor, limit int) ([]*OutboxMessage, error) {
	var rows []outboxRow
	err := sqlx.SelectContext(ctx, ex, &rows, `
		UPDATE email_outbox
		SET status = 'sending', locked_until = now() + interval '2 minutes'
		WHERE id IN (
		  SELECT id FROM email_outbox
		  WHERE (status = 'queued' AND run_at <= now())
		     OR (status = 'sending' AND locked_until < now())
		  ORDER BY run_at
		  LIMIT $1
		  FOR UPDATE SKIP LOCKED
		)
		RETURNING `+outboxColumns, limit)
	if err != nil {
		return nil, fmt.Errorf("mailer: claim: %w", err)
	}
	out := make([]*OutboxMessage, 0, len(rows))
	for _, r := range rows {
		m, err := r.toMessage()
		if err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, nil
}

// MarkSent đặt sent, sent_at và xóa secret_enc.
func (PGOutboxRepo) MarkSent(ctx context.Context, ex db.Executor, id uuid.UUID, now time.Time) error {
	err := db.ExecAffectOne(ctx, ex, `
		UPDATE email_outbox SET status = 'sent', sent_at = $2, secret_enc = NULL, locked_until = NULL WHERE id = $1`, id, now)
	if err != nil {
		return fmt.Errorf("mailer: mark sent %s: %w", id, err)
	}
	return nil
}

// MarkFailedAttempt ghi kết quả đã tính ở entity.
func (PGOutboxRepo) MarkFailedAttempt(ctx context.Context, ex db.Executor, m *OutboxMessage) error {
	err := db.ExecAffectOne(ctx, ex, `
		UPDATE email_outbox
		SET status = $2, attempts = $3, run_at = $4, last_error = $5, secret_enc = $6, locked_until = NULL
		WHERE id = $1`,
		m.ID, m.Status, m.Attempts, m.RunAt, m.LastError, m.SecretEnc)
	if err != nil {
		return fmt.Errorf("mailer: mark failed %s: %w", m.ID, err)
	}
	return nil
}

// StatusByIDs trả trạng thái của các id có trong bảng; id không có thì không có khóa trong map.
func (PGOutboxRepo) StatusByIDs(ctx context.Context, ex db.Executor, ids []uuid.UUID) (map[uuid.UUID]DeliveryStatus, error) {
	out := make(map[uuid.UUID]DeliveryStatus, len(ids))
	if len(ids) == 0 {
		return out, nil
	}
	var rows []struct {
		ID        uuid.UUID  `db:"id"`
		Status    string     `db:"status"`
		Attempts  int        `db:"attempts"`
		LastError *string    `db:"last_error"`
		SentAt    *time.Time `db:"sent_at"`
	}
	arr := pgtype.FlatArray[uuid.UUID](ids)
	if err := sqlx.SelectContext(ctx, ex, &rows,
		`SELECT id, status, attempts, last_error, sent_at FROM email_outbox WHERE id = ANY($1)`, arr); err != nil {
		return nil, fmt.Errorf("mailer: status by ids: %w", err)
	}
	for _, r := range rows {
		out[r.ID] = DeliveryStatus{Status: r.Status, Attempts: r.Attempts, LastError: r.LastError, SentAt: r.SentAt}
	}
	return out, nil
}

// SupersedeQueued đánh dấu failed các hàng queued cũ của toEmail thuộc templates, xóa bí mật.
func (PGOutboxRepo) SupersedeQueued(ctx context.Context, ex db.Executor, toEmail string, templates []string, _ time.Time) (int64, error) {
	if len(templates) == 0 {
		return 0, errors.New("mailer: supersede cần ít nhất một template")
	}
	res, err := ex.ExecContext(ctx, `
		UPDATE email_outbox SET status = 'failed', last_error = 'superseded', secret_enc = NULL, locked_until = NULL
		WHERE lower(to_email) = lower($1) AND template = ANY($2) AND status = 'queued'`,
		toEmail, pgtype.FlatArray[string](templates))
	if err != nil {
		return 0, fmt.Errorf("mailer: supersede: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("mailer: supersede rows: %w", err)
	}
	return n, nil
}
