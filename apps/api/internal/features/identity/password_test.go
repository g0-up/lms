package identity

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"log/slog"
	"strings"
	"testing"
	"testing/iotest"

	"github.com/alexedwards/argon2id"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPasswordHashUsesProjectArgon2Params(t *testing.T) {
	h, err := NewPasswordHash("matkhau-dung-1")
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(h.PHC(), "$argon2id$v=19$m=19456,t=2,p=1$"), h.PHC())

	params, salt, key, err := argon2id.DecodeHash(h.PHC())
	require.NoError(t, err)
	assert.Equal(t, uint32(19*1024), params.Memory)
	assert.Equal(t, uint32(2), params.Iterations)
	assert.Equal(t, uint8(1), params.Parallelism)
	assert.Len(t, salt, 16)
	assert.Len(t, key, 32)
}

func TestPasswordHashRoundTrip(t *testing.T) {
	h, err := NewPasswordHash("matkhau-dung-1")
	require.NoError(t, err)
	assert.True(t, h.Verify("matkhau-dung-1"))
	assert.False(t, h.Verify("matkhau-sai"))

	again := PasswordHashFromPHC(h.PHC())
	assert.True(t, again.Verify("matkhau-dung-1"))

	other, err := NewPasswordHash("matkhau-dung-1")
	require.NoError(t, err)
	assert.NotEqual(t, h.PHC(), other.PHC(), "salt ngẫu nhiên nên hai hash phải khác nhau")
}

func TestPasswordHashVerifyRejectsBrokenHash(t *testing.T) {
	assert.False(t, PasswordHash{}.Verify(""))
	assert.False(t, PasswordHashFromPHC("not-a-phc").Verify("x"))
	assert.False(t, PasswordHashFromPHC("$argon2id$v=19$m=19456,t=2,p=1$bad$bad").Verify("x"))
}

func TestDummyHashIsStableAndNeverMatchesInput(t *testing.T) {
	assert.Equal(t, dummyHash().PHC(), dummyHash().PHC())
	assert.False(t, dummyHash().Verify("matkhau-dung-1"))
}

func TestGenerateTemporaryPassword(t *testing.T) {
	seen := map[string]bool{}
	for range 50 {
		tmp, err := GenerateTemporaryPassword(rand.Reader, TemporaryPasswordLength)
		require.NoError(t, err)
		v := tmp.Reveal()
		require.Len(t, v, TemporaryPasswordLength)
		for _, r := range v {
			require.True(t, strings.ContainsRune(tempAlphabet, r), "ký tự %q ngoài bảng chữ", r)
		}
		require.False(t, seen[v], "mật khẩu tạm bị trùng")
		seen[v] = true
	}
	assert.NotContains(t, tempAlphabet, "0")
	assert.NotContains(t, tempAlphabet, "1")
	for _, c := range "IOlo" {
		assert.NotContains(t, tempAlphabet, string(c))
	}

	long, err := GenerateTemporaryPassword(rand.Reader, 20)
	require.NoError(t, err)
	assert.Len(t, long.Reveal(), 20)
}

func TestGenerateTemporaryPasswordRejectsShortLengthAndRandomFailure(t *testing.T) {
	_, err := GenerateTemporaryPassword(rand.Reader, TemporaryPasswordLength-1)
	require.ErrorIs(t, err, errTemporaryPasswordLength)

	_, err = GenerateTemporaryPassword(iotest.ErrReader(assert.AnError), TemporaryPasswordLength)
	require.ErrorIs(t, err, assert.AnError)
}

func TestGenerateTemporaryPasswordSkipsBiasedBytes(t *testing.T) {
	// 255 nằm ngoài vùng lấy mẫu (256 - 256%56 = 224) nên bị bỏ; byte 0 ánh xạ tới ký tự đầu bảng chữ.
	src := bytes.Repeat([]byte{255, 0}, TemporaryPasswordLength*2)
	tmp, err := GenerateTemporaryPassword(bytes.NewReader(src), TemporaryPasswordLength)
	require.NoError(t, err)
	assert.Equal(t, strings.Repeat(tempAlphabet[:1], TemporaryPasswordLength), tmp.Reveal())
}

func TestSecretsAreMaskedInFmtAndSlog(t *testing.T) {
	tmp, err := GenerateTemporaryPassword(rand.Reader, TemporaryPasswordLength)
	require.NoError(t, err)
	sess, err := NewSessionToken(rand.Reader)
	require.NoError(t, err)
	reset, err := NewResetToken(rand.Reader)
	require.NoError(t, err)

	for name, v := range map[string]interface {
		Reveal() string
		IsZero() bool
	}{"temp": tmp, "session": sess, "reset": reset} {
		raw := v.Reveal()
		require.NotEmpty(t, raw, name)
		assert.False(t, v.IsZero(), name)
		for _, verb := range []string{"%v", "%+v", "%s", "%q", "%#v", "%x"} {
			out := fmt.Sprintf(verb, v)
			assert.NotContains(t, out, raw, "%s %s", name, verb)
			assert.Contains(t, out, "***", "%s %s", name, verb)
		}
		var buf bytes.Buffer
		slog.New(slog.NewJSONHandler(&buf, nil)).Info("x", "secret", v)
		assert.NotContains(t, buf.String(), raw, name)
		assert.Contains(t, buf.String(), "***", name)
	}
	assert.True(t, TemporaryPassword{}.IsZero())
}

func TestOpaqueTokensAreRandomAndHashedWithSHA256(t *testing.T) {
	a, err := NewSessionToken(rand.Reader)
	require.NoError(t, err)
	b, err := NewSessionToken(rand.Reader)
	require.NoError(t, err)
	assert.NotEqual(t, a.Reveal(), b.Reveal())

	raw, err := base64.RawURLEncoding.DecodeString(a.Reveal())
	require.NoError(t, err)
	assert.Len(t, raw, tokenBytes)

	sum := sha256.Sum256([]byte(a.Reveal()))
	assert.Equal(t, sum[:], a.Hash())
	assert.Equal(t, a.Hash(), HashToken(a.Reveal()))

	r, err := NewResetToken(rand.Reader)
	require.NoError(t, err)
	assert.Equal(t, HashToken(r.Reveal()), r.Hash())

	_, err = NewSessionToken(iotest.ErrReader(assert.AnError))
	require.ErrorIs(t, err, errShortRandom)
	_, err = NewResetToken(bytes.NewReader(make([]byte, tokenBytes-1)))
	require.ErrorIs(t, err, errShortRandom)
}
