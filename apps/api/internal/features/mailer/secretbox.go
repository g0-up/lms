package mailer

import (
	"encoding/base64"
	"errors"

	"lms/api/internal/platform/secretbox"
)

// errSecretKey không chứa giá trị khóa để log không lộ secret.
var errSecretKey = errors.New("mailer: OUTBOX_SECRET_KEY phải là base64 của 32 byte")

// NewSecretBox dựng secretbox.Box từ OUTBOX_SECRET_KEY (base64 chuẩn của 32 byte). API, worker và seed dùng
// chung để cùng một khóa niêm phong và mở secret_enc.
func NewSecretBox(keyBase64 string) (*secretbox.Box, error) {
	key, err := base64.StdEncoding.DecodeString(keyBase64)
	if err != nil {
		return nil, errSecretKey
	}
	box, err := secretbox.New(key)
	if err != nil {
		return nil, errSecretKey
	}
	return box, nil
}
