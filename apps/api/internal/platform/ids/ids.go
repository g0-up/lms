// Package ids sinh và đọc khóa chính uuid v7 ở tầng ứng dụng (schema không có DEFAULT cho cột id).
package ids

import (
	"errors"

	"github.com/google/uuid"
)

// ErrInvalid báo chuỗi không phải uuid dạng chuẩn.
var ErrInvalid = errors.New("ids: uuid không hợp lệ")

// New trả một uuid v7: tăng dần theo thời gian nên index B-tree của PK ít phân mảnh.
func New() uuid.UUID {
	return uuid.Must(uuid.NewV7())
}

// Parse đọc uuid ở dạng chuẩn 36 ký tự (xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx), từ chối các dạng khác mà
// uuid.Parse chấp nhận (urn:uuid:, {…}, 32 hex) để mỗi id chỉ có một cách viết trong URL.
func Parse(s string) (uuid.UUID, error) {
	if len(s) != 36 {
		return uuid.Nil, ErrInvalid
	}
	id, err := uuid.Parse(s)
	if err != nil {
		return uuid.Nil, ErrInvalid
	}
	return id, nil
}

// Generator sinh khóa chính; service nhận Generator để test cố định được id.
type Generator interface {
	New() uuid.UUID
}

// V7 là Generator thật, gọi New.
type V7 struct{}

var _ Generator = V7{}

// New trả một uuid v7.
func (V7) New() uuid.UUID { return New() }
