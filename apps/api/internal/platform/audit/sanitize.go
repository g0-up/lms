package audit

import (
	"encoding/json"
	"fmt"
	"strings"
)

// sensitiveKeys là khóa bị xóa khỏi before/after, so sánh sau khi bỏ "_" và chữ hoa nên bắt cả passwordHash,
// PasswordHash, tempPassword.
var sensitiveKeys = map[string]struct{}{
	"passwordhash": {},
	"tokenhash":    {},
	"password":     {},
	"temppassword": {},
}

func isSensitive(key string) bool {
	_, ok := sensitiveKeys[strings.ToLower(strings.ReplaceAll(key, "_", ""))]
	return ok
}

// sanitize đưa v qua JSON (để struct theo tag json thành map) rồi xóa đệ quy khóa nhạy cảm trong map và slice.
func sanitize(v any) (any, error) {
	raw, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("audit: marshal: %w", err)
	}
	var out any
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("audit: unmarshal: %w", err)
	}
	return strip(out), nil
}

func strip(v any) any {
	switch t := v.(type) {
	case map[string]any:
		for k, child := range t {
			if isSensitive(k) {
				delete(t, k)
				continue
			}
			t[k] = strip(child)
		}
		return t
	case []any:
		for i, child := range t {
			t[i] = strip(child)
		}
		return t
	default:
		return v
	}
}

// jsonb trả JSON đã làm sạch của v, hoặc nil (NULL) khi v là nil.
func jsonb(v any) (any, error) {
	if v == nil {
		return nil, nil
	}
	clean, err := sanitize(v)
	if err != nil {
		return nil, err
	}
	if clean == nil {
		return nil, nil
	}
	raw, err := json.Marshal(clean)
	if err != nil {
		return nil, fmt.Errorf("audit: marshal: %w", err)
	}
	return string(raw), nil
}
