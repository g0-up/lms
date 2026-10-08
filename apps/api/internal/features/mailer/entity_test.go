package mailer

import (
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var entityNow = time.Date(2026, 10, 5, 9, 0, 0, 0, time.UTC)

func TestMarkFailedAttemptBacksOffThenFails(t *testing.T) {
	m := &OutboxMessage{Status: StatusSending, SecretEnc: []byte("sealed"), RunAt: entityNow}
	lease := entityNow.Add(2 * time.Minute)
	m.LockedUntil = &lease

	require.False(t, m.MarkFailedAttempt("smtp: 421", entityNow))
	assert.Equal(t, 1, m.Attempts)
	assert.Equal(t, StatusQueued, m.Status)
	assert.Equal(t, entityNow.Add(time.Minute), m.RunAt)
	assert.Equal(t, "smtp: 421", *m.LastError)
	assert.Nil(t, m.LockedUntil)
	assert.NotNil(t, m.SecretEnc, "còn lượt gửi thì giữ bí mật")

	second := entityNow.Add(time.Minute)
	require.False(t, m.MarkFailedAttempt("smtp: 421", second))
	assert.Equal(t, 2, m.Attempts)
	assert.Equal(t, second.Add(5*time.Minute), m.RunAt)

	third := second.Add(5 * time.Minute)
	require.True(t, m.MarkFailedAttempt("smtp: 550", third))
	assert.Equal(t, MaxAttempts, m.Attempts)
	assert.Equal(t, StatusFailed, m.Status)
	assert.Equal(t, "smtp: 550", *m.LastError)
	assert.Nil(t, m.SecretEnc, "thất bại lần cuối xóa bí mật")
}

func TestMarkFailedAttemptTruncatesLastError(t *testing.T) {
	m := &OutboxMessage{}
	m.MarkFailedAttempt(strings.Repeat("ắ", maxLastError+50), entityNow)
	assert.Equal(t, maxLastError, utf8.RuneCountInString(*m.LastError))
	assert.True(t, utf8.ValidString(*m.LastError))
	assert.Equal(t, "ngắn", truncateRunes("ngắn", 10))
}

func TestMarkSentClearsSecret(t *testing.T) {
	lease := entityNow
	m := &OutboxMessage{Status: StatusSending, SecretEnc: []byte("sealed"), LockedUntil: &lease}
	m.MarkSent(entityNow)
	assert.Equal(t, StatusSent, m.Status)
	assert.Equal(t, entityNow, *m.SentAt)
	assert.Nil(t, m.SecretEnc)
	assert.Nil(t, m.LockedUntil)
}
