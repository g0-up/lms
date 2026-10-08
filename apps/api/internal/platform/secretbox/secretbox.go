// Package secretbox mã hóa bí mật ngắn hạn (mật khẩu tạm, token đặt lại) lưu trong email_outbox.secret_enc
// bằng AES-256-GCM với khóa OUTBOX_SECRET_KEY. Gói không log và không có String() để plaintext không lọt ra.
package secretbox

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"fmt"
)

// KeySize là độ dài khóa AES-256.
const KeySize = 32

// ErrSecretInvalid báo dữ liệu đã mã hóa bị sửa, bị cắt hoặc mã hóa bằng khóa khác.
var ErrSecretInvalid = errors.New("secretbox: dữ liệu mã hóa không hợp lệ")

// Box mã hóa và giải mã bằng một khóa cố định. An toàn khi dùng từ nhiều goroutine.
type Box struct {
	aead cipher.AEAD
}

// New tạo Box từ khóa 32 byte (cfg.OutboxSecretKey đã decode base64).
func New(key []byte) (*Box, error) {
	if len(key) != KeySize {
		return nil, fmt.Errorf("secretbox: khóa phải dài %d byte, nhận %d", KeySize, len(key))
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("secretbox: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("secretbox: %w", err)
	}
	return &Box{aead: aead}, nil
}

// Seal trả nonce ngẫu nhiên 12 byte || ciphertext+tag. Hai lần Seal cùng plaintext cho kết quả khác nhau.
func (b *Box) Seal(plaintext []byte) ([]byte, error) {
	nonce := make([]byte, b.aead.NonceSize(), b.aead.NonceSize()+len(plaintext)+b.aead.Overhead())
	if _, err := rand.Read(nonce); err != nil {
		return nil, fmt.Errorf("secretbox: sinh nonce: %w", err)
	}
	return b.aead.Seal(nonce, nonce, plaintext, nil), nil
}

// Open giải mã kết quả của Seal; dữ liệu bị sửa, quá ngắn hoặc sai khóa trả ErrSecretInvalid.
func (b *Box) Open(sealed []byte) ([]byte, error) {
	ns := b.aead.NonceSize()
	if len(sealed) < ns+b.aead.Overhead() {
		return nil, ErrSecretInvalid
	}
	plaintext, err := b.aead.Open(nil, sealed[:ns], sealed[ns:], nil)
	if err != nil {
		return nil, ErrSecretInvalid
	}
	return plaintext, nil
}
