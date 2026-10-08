package config

import (
	"bufio"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// composeOnlyKeys là biến chỉ compose/Caddy/test đọc, không thuộc Config.
var composeOnlyKeys = map[string]bool{
	"DOMAIN":            true,
	"API_DOMAIN":        true,
	"ACME_EMAIL":        true,
	"MEDIA_ORIGIN":      true,
	"TEST_DATABASE_URL": true,
}

const composeOnlyPrefix = "POSTGRES_"

var envKeyPattern = regexp.MustCompile(`^[A-Z][A-Z0-9_]*$`)

// configEnvKeys trả tập tên biến khai báo bằng tag `env:` trên Config (bỏ field `env:"-"`).
func configEnvKeys(t *testing.T) map[string]bool {
	t.Helper()
	keys := map[string]bool{}
	typ := reflect.TypeFor[Config]()
	for i := range typ.NumField() {
		tag := typ.Field(i).Tag.Get("env")
		name, _, _ := strings.Cut(tag, ",")
		if name == "" || name == "-" {
			continue
		}
		keys[name] = true
	}
	require.NotEmpty(t, keys)
	return keys
}

// parseEnvExample đọc khóa của file dạng dotenv; báo lỗi dòng sai cú pháp và khóa trùng.
func parseEnvExample(t *testing.T, path string) []string {
	t.Helper()
	f, err := os.Open(path)
	require.NoError(t, err)
	defer func() { _ = f.Close() }()

	var keys []string
	seen := map[string]bool{}
	sc := bufio.NewScanner(f)
	for line := 1; sc.Scan(); line++ {
		text := strings.TrimSpace(sc.Text())
		if text == "" || strings.HasPrefix(text, "#") {
			continue
		}
		key, _, ok := strings.Cut(text, "=")
		key = strings.TrimSpace(key)
		require.Truef(t, ok && envKeyPattern.MatchString(key), "%s:%d không phải dòng KEY=value: %q", path, line, text)
		require.Falsef(t, seen[key], "%s:%d khóa %s bị lặp", path, line, key)
		seen[key] = true
		keys = append(keys, key)
	}
	require.NoError(t, sc.Err())
	return keys
}

// TestEnvExample giữ file mẫu không trôi khỏi Config: mọi khóa phải là tag `env:` hoặc biến compose-only.
func TestEnvExample(t *testing.T) {
	known := configEnvKeys(t)
	files := []string{
		filepath.Join("..", "..", "..", "..", "..", "infra", ".env.example"),
		filepath.Join("..", "..", "..", ".env.example"),
	}
	for _, path := range files {
		t.Run(path, func(t *testing.T) {
			keys := parseEnvExample(t, path)
			require.NotEmpty(t, keys)
			for _, k := range keys {
				ok := known[k] || composeOnlyKeys[k] || strings.HasPrefix(k, composeOnlyPrefix)
				assert.Truef(t, ok, "%s: khóa %s không có trong Config và không thuộc allowlist compose-only", path, k)
			}
		})
	}
}
