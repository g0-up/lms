package audit

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"lms/api/internal/platform/clock"
)

type userSnapshot struct {
	Email        string `json:"email"`
	Status       string `json:"status"`
	PasswordHash string `json:"password_hash"`
	TempPassword string `json:"tempPassword"`
}

func TestSanitizeStripsSensitiveKeysRecursively(t *testing.T) {
	in := map[string]any{
		"status":        "disabled",
		"password_hash": "$argon2id$...",
		"profile": map[string]any{
			"token_hash": "abc",
			"Password":   "x",
			"name":       "An",
		},
		"history": []any{
			map[string]any{"passwordHash": "y", "at": "2026-10-05"},
			[]any{map[string]any{"temp_password": "z", "ok": true}},
			"plain",
		},
	}
	got, err := sanitize(in)
	require.NoError(t, err)
	require.Equal(t, map[string]any{
		"status":  "disabled",
		"profile": map[string]any{"name": "An"},
		"history": []any{
			map[string]any{"at": "2026-10-05"},
			[]any{map[string]any{"ok": true}},
			"plain",
		},
	}, got)

	got, err = sanitize(userSnapshot{Email: "an@goup.vn", Status: "active", PasswordHash: "h", TempPassword: "t"})
	require.NoError(t, err)
	require.Equal(t, map[string]any{"email": "an@goup.vn", "status": "active"}, got)

	got, err = sanitize("chuỗi")
	require.NoError(t, err)
	require.Equal(t, "chuỗi", got)

	_, err = sanitize(make(chan int))
	require.Error(t, err)
}

func TestJSONB(t *testing.T) {
	v, err := jsonb(nil)
	require.NoError(t, err)
	require.Nil(t, v)

	var nilMap map[string]any
	v, err = jsonb(nilMap)
	require.NoError(t, err)
	require.Nil(t, v, "map nil marshal thành null → NULL")

	v, err = jsonb(map[string]any{"status": "active", "password": "x"})
	require.NoError(t, err)
	require.JSONEq(t, `{"status":"active"}`, v.(string))

	_, err = jsonb(func() {})
	require.Error(t, err)
}

func TestPGRecordValidatesBeforeTouchingDatabase(t *testing.T) {
	p := PG{Clock: &clock.Fake{T: time.Now()}}
	ctx := context.Background()
	id := uuid.Must(uuid.NewV7())

	err := p.Record(ctx, nil, Entry{Action: "user.invite", TargetType: "user", TargetID: &id})
	require.ErrorContains(t, err, "action không thuộc danh sách")

	err = p.Record(ctx, nil, Entry{Action: ActionUserDisabled, TargetID: &id})
	require.ErrorContains(t, err, "thiếu target_type")

	err = PG{}.Record(ctx, nil, Entry{Action: ActionUserDisabled, TargetType: "user"})
	require.ErrorContains(t, err, "thiếu Clock")

	err = p.Record(ctx, nil, Entry{Action: ActionUserDisabled, TargetType: "user", Before: make(chan int)})
	require.Error(t, err)
	err = p.Record(ctx, nil, Entry{Action: ActionUserDisabled, TargetType: "user", After: make(chan int)})
	require.Error(t, err)
}

func TestNoop(t *testing.T) {
	var r Recorder = Noop{}
	require.NoError(t, r.Record(context.Background(), nil, Entry{Action: "anything"}))
}

func TestEntryJSONShapeIsStable(t *testing.T) {
	// Before/After nhận struct của feature; bảo đảm tag json được giữ (không dùng tên field Go).
	got, err := sanitize(struct {
		CourseVersionID string `json:"courseVersionId"`
	}{"cv-1"})
	require.NoError(t, err)
	raw, err := json.Marshal(got)
	require.NoError(t, err)
	require.JSONEq(t, `{"courseVersionId":"cv-1"}`, string(raw))
}
