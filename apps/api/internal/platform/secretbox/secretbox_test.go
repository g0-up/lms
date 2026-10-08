package secretbox_test

import (
	"bytes"
	"crypto/rand"
	"testing"

	"github.com/stretchr/testify/require"

	"lms/api/internal/platform/secretbox"
)

func newKey(t *testing.T) []byte {
	t.Helper()
	key := make([]byte, secretbox.KeySize)
	_, err := rand.Read(key)
	require.NoError(t, err)
	return key
}

func TestNewRejectsWrongKeySize(t *testing.T) {
	for _, n := range []int{0, 16, 24, 31, 33, 64} {
		box, err := secretbox.New(make([]byte, n))
		require.Error(t, err, "khóa %d byte", n)
		require.Nil(t, box)
	}
}

func TestSealOpenRoundTrip(t *testing.T) {
	box, err := secretbox.New(newKey(t))
	require.NoError(t, err)

	for _, plain := range [][]byte{[]byte("Tm9-mật-khẩu-tạm"), {}, bytes.Repeat([]byte("x"), 4096)} {
		sealed, err := box.Seal(plain)
		require.NoError(t, err)
		require.Len(t, sealed, 12+len(plain)+16, "nonce 12 byte || ciphertext || tag 16 byte")
		if len(plain) > 0 {
			require.NotContains(t, string(sealed), string(plain))
		}
		got, err := box.Open(sealed)
		require.NoError(t, err)
		require.Equal(t, string(plain), string(got))
	}
}

func TestSealUsesFreshNonce(t *testing.T) {
	box, err := secretbox.New(newKey(t))
	require.NoError(t, err)
	a, err := box.Seal([]byte("cùng nội dung"))
	require.NoError(t, err)
	b, err := box.Seal([]byte("cùng nội dung"))
	require.NoError(t, err)
	require.NotEqual(t, a, b)
	require.NotEqual(t, a[:12], b[:12])
}

func TestOpenDetectsTampering(t *testing.T) {
	box, err := secretbox.New(newKey(t))
	require.NoError(t, err)
	sealed, err := box.Seal([]byte("token-đặt-lại"))
	require.NoError(t, err)

	for i := range sealed {
		tampered := bytes.Clone(sealed)
		tampered[i] ^= 0x01
		_, err := box.Open(tampered)
		require.ErrorIs(t, err, secretbox.ErrSecretInvalid, "đổi byte %d", i)
	}
	for _, short := range [][]byte{nil, sealed[:11], sealed[:27]} {
		_, err := box.Open(short)
		require.ErrorIs(t, err, secretbox.ErrSecretInvalid)
	}
}

func TestOpenWithOtherKeyFails(t *testing.T) {
	a, err := secretbox.New(newKey(t))
	require.NoError(t, err)
	b, err := secretbox.New(newKey(t))
	require.NoError(t, err)
	sealed, err := a.Seal([]byte("bí mật"))
	require.NoError(t, err)
	_, err = b.Open(sealed)
	require.ErrorIs(t, err, secretbox.ErrSecretInvalid)
}
