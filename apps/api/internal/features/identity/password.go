package identity

import (
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"sync"

	"github.com/alexedwards/argon2id"
)

// TemporaryPasswordLength là độ dài mật khẩu tạm (spec: tối thiểu 12 ký tự); hằng, không có biến env.
const TemporaryPasswordLength = 12

// tempAlphabet là [A-Za-z2-9] bỏ ký tự dễ nhầm khi đọc trong email: I, O, l, o (0 và 1 vốn không có).
const tempAlphabet = "ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnpqrstuvwxyz23456789"

// tokenBytes là số byte ngẫu nhiên của token phiên và token đặt lại mật khẩu.
const tokenBytes = 32

// passwordParams là tham số argon2id (OWASP) DUY NHẤT của dự án; seed hash qua NewPasswordHash nên dùng chung.
var passwordParams = &argon2id.Params{Memory: 19 * 1024, Iterations: 2, Parallelism: 1, SaltLength: 16, KeyLength: 32}

var errTemporaryPasswordLength = fmt.Errorf("identity: mật khẩu tạm phải có ít nhất %d ký tự", TemporaryPasswordLength)

// secret giữ một chuỗi bí mật và che nó khỏi fmt, slog: mọi verb của fmt và LogValue đều in "***".
// Giá trị thật chỉ lấy qua Reveal ở đúng chỗ cần (payload email, cookie).
type secret struct{ v string }

// Reveal trả giá trị thật; chỉ gọi khi đưa bí mật ra khỏi tiến trình (secret_enc của outbox, Set-Cookie).
func (s secret) Reveal() string { return s.v }

// IsZero cho biết chưa có giá trị.
func (s secret) IsZero() bool { return s.v == "" }

// Format in "***" cho mọi verb (kể cả %#v) để bí mật không lọt vào log qua fmt.
func (secret) Format(f fmt.State, _ rune) { _, _ = io.WriteString(f, "***") }

// LogValue in "***" khi bí mật bị đưa vào slog.
func (secret) LogValue() slog.Value { return slog.StringValue("***") }

// TemporaryPassword là mật khẩu tạm sinh bằng CSPRNG; không có MarshalJSON, fmt và slog in "***".
type TemporaryPassword struct{ secret }

// GenerateTemporaryPassword sinh mật khẩu length ký tự từ tempAlphabet, đọc byte từ rnd (crypto/rand.Reader khi
// chạy thật). Lấy mẫu loại bỏ (rejection sampling) để mọi ký tự có xác suất như nhau.
func GenerateTemporaryPassword(rnd io.Reader, length int) (TemporaryPassword, error) {
	if length < TemporaryPasswordLength {
		return TemporaryPassword{}, errTemporaryPasswordLength
	}
	n := len(tempAlphabet)
	limit := 256 - 256%n
	out := make([]byte, 0, length)
	buf := make([]byte, length*2)
	for len(out) < length {
		if _, err := io.ReadFull(rnd, buf); err != nil {
			return TemporaryPassword{}, fmt.Errorf("identity: sinh mật khẩu tạm: %w", err)
		}
		for _, b := range buf {
			if int(b) >= limit {
				continue
			}
			out = append(out, tempAlphabet[int(b)%n])
			if len(out) == length {
				break
			}
		}
	}
	return TemporaryPassword{secret{string(out)}}, nil
}

// PasswordHash là chuỗi PHC argon2id ($argon2id$v=19$m=…,t=…,p=…$salt$key). DB chỉ lưu giá trị này.
type PasswordHash struct{ phc string }

// NewPasswordHash hash plain bằng argon2id với passwordParams.
func NewPasswordHash(plain string) (PasswordHash, error) {
	phc, err := argon2id.CreateHash(plain, passwordParams)
	if err != nil {
		return PasswordHash{}, fmt.Errorf("identity: hash mật khẩu: %w", err)
	}
	return PasswordHash{phc: phc}, nil
}

// PasswordHashFromPHC bọc chuỗi PHC đọc từ DB.
func PasswordHashFromPHC(phc string) PasswordHash { return PasswordHash{phc: phc} }

// PHC trả chuỗi để lưu cột users.password_hash.
func (h PasswordHash) PHC() string { return h.phc }

// Verify so plain với hash; hash hỏng hoặc rỗng coi như sai.
func (h PasswordHash) Verify(plain string) bool {
	if h.phc == "" {
		return false
	}
	ok, err := argon2id.ComparePasswordAndHash(plain, h.phc)
	return err == nil && ok
}

// dummyHash là hash cố định để verify khi email không tồn tại, giữ thời gian phản hồi như khi có user.
var dummyHash = sync.OnceValue(func() PasswordHash {
	h, err := NewPasswordHash("goup-lms-timing-equalizer")
	if err != nil {
		panic(err) // chỉ hỏng khi crypto/rand hỏng; tiến trình không thể chạy an toàn
	}
	return h
})

// SessionToken là token phiên opaque (32 byte ngẫu nhiên, base64url) gửi trong cookie; DB chỉ lưu Hash.
type SessionToken struct{ secret }

// NewSessionToken sinh token phiên từ rnd.
func NewSessionToken(rnd io.Reader) (SessionToken, error) {
	raw, err := newOpaqueToken(rnd)
	return SessionToken{secret{raw}}, err
}

// Hash là SHA-256 của token, giá trị lưu ở sessions.token_hash.
func (t SessionToken) Hash() []byte { return HashToken(t.v) }

// ResetToken là token đặt lại mật khẩu dùng một lần, cùng định dạng SessionToken.
type ResetToken struct{ secret }

// NewResetToken sinh token đặt lại mật khẩu từ rnd.
func NewResetToken(rnd io.Reader) (ResetToken, error) {
	raw, err := newOpaqueToken(rnd)
	return ResetToken{secret{raw}}, err
}

// Hash là SHA-256 của token, giá trị lưu ở password_reset_tokens.token_hash.
func (t ResetToken) Hash() []byte { return HashToken(t.v) }

// HashToken là SHA-256 (32 byte) của chuỗi token nhận từ client (cookie hoặc body).
func HashToken(raw string) []byte {
	sum := sha256.Sum256([]byte(raw))
	return sum[:]
}

var errShortRandom = errors.New("identity: thiếu byte ngẫu nhiên")

func newOpaqueToken(rnd io.Reader) (string, error) {
	b := make([]byte, tokenBytes)
	if _, err := io.ReadFull(rnd, b); err != nil {
		return "", errors.Join(errShortRandom, err)
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
