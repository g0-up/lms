package seed

import (
	"bytes"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"lms/api/internal/platform/config"
)

func TestParseDuration(t *testing.T) {
	cases := []struct {
		in   string
		want int
		ok   bool
	}{
		{"18:24", 1104, true},
		{"0:59", 59, true},
		{"1:02:03", 3723, true},
		{"28:40", 1720, true},
		{"18", 0, false},
		{"18:6", 0, false},
		{"18:60", 0, false},
		{"-1:00", 0, false},
		{"a:00", 0, false},
		{"", 0, false},
	}
	for _, c := range cases {
		got, err := parseDuration(c.in)
		if !c.ok {
			assert.Error(t, err, c.in)
			continue
		}
		require.NoError(t, err, c.in)
		assert.Equal(t, c.want, got, c.in)
	}
}

// TestDatasetMatchesPrototype ghim số lượng của prototype/seed.js để lỗi chép dữ liệu hiện ra ngay.
func TestDatasetMatchesPrototype(t *testing.T) {
	require.NoError(t, checkData())

	assert.Len(t, users, 17)
	assert.Len(t, stages, 5)
	assert.Len(t, stageVersions, 5)
	assert.Equal(t, 16, lessonCount())
	assert.Equal(t, 9, videoLessonCount())
	assert.Len(t, courses, 1)
	assert.Len(t, courseVersions, 1)
	assert.Len(t, classes, 3)
	assert.Len(t, enrollments, 15, "basic01: 8 (gồm Phong vào muộn và Thảo đã rời), basic02: 5, basic03: 2")
	assert.Len(t, audits, 10)

	byStatus := map[string]int{}
	for _, e := range enrollments {
		byStatus[e.invite]++
	}
	assert.Equal(t, map[string]int{inviteSent: 13, inviteFailed: 1, inviteQueued: 1}, byStatus)

	var first, last userSeed
	for _, u := range users {
		if u.key == adminKey {
			first = u
		}
		if u.key == "u-thao" {
			last = u
		}
	}
	assert.Equal(t, markerEmail, first.email)
	assert.Equal(t, "admin", first.role)
	assert.Equal(t, "disabled", last.status)
}

func lessonCount() int {
	n := 0
	for _, sv := range stageVersions {
		n += len(sv.lessons)
	}
	return n
}

func videoLessonCount() int {
	n := 0
	for _, sv := range stageVersions {
		for _, l := range sv.lessons {
			if l.typ == "video" {
				n++
			}
		}
	}
	return n
}

func progressCount() int {
	n := 0
	for _, e := range enrollments {
		n += len(e.done) + len(e.opened)
	}
	return n
}

func TestEveryInvitedUserHasTemporaryPasswordExpiry(t *testing.T) {
	for _, u := range users {
		invited := u.status == "invited"
		assert.Equal(t, invited, u.mustChangePassword, u.key)
		assert.Equal(t, invited, u.tempPasswordExpiresAt != nil, u.key)
		if invited {
			assert.Nil(t, u.lastLoginAt, u.key)
		}
		assert.Equal(t, normalizeEmail(u.email), u.email, "email trong seed.js đã ở dạng chuẩn hóa: %s", u.key)
	}
}

var allowedTag = regexp.MustCompile(`^(h1|h2|h3|h4|p|br|ul|ol|li|strong|em|del|code|pre|blockquote|hr|table|thead|tbody|tr|th|td)$`)

func TestMarkdownDocs(t *testing.T) {
	tags := regexp.MustCompile(`<(/?)([a-zA-Z0-9]+)([^>]*)>`)
	for _, sv := range stageVersions {
		for _, l := range sv.lessons {
			if l.markdown == nil {
				continue
			}
			md := l.markdown
			assert.NotContains(t, md.source, "ˋ", "%s: còn ký tự thay backtick", l.key)
			assert.True(t, strings.HasPrefix(md.source, "# "), l.key)
			require.NotEmpty(t, md.html, l.key)

			for _, m := range tags.FindAllStringSubmatch(md.html, -1) {
				assert.Regexp(t, allowedTag, m[2], "%s: thẻ ngoài allowlist", l.key)
				if m[3] != "" {
					assert.Regexp(t, `^ class="language-[a-z0-9]+"$`, m[3], "%s: thuộc tính ngoài allowlist", l.key)
				}
			}
			title := strings.TrimPrefix(strings.SplitN(md.source, "\n", 2)[0], "# ")
			assert.Contains(t, md.html, "<h1>"+title+"</h1>", l.key)
		}
	}
	assert.Contains(t, mdTableDesign.source, "`ON DELETE RESTRICT`")
	assert.Contains(t, mdTableDesign.source, "```sql\nCREATE TABLE class_members (")
}

// update khai báo ở file không có build tag để TestSeedMarkdownGolden chạy -update mà không cần DB.
var update = flag.Bool("update", false, "ghi lại golden JSON trong testdata/")

// TestSeedMarkdownGolden ghim nguồn mọi bài markdown seed vào testdata/seed_markdown.json; web dùng file này để
// kiểm trình soạn giữ nguyên nội dung seed thật.
func TestSeedMarkdownGolden(t *testing.T) {
	type entry struct {
		Key    string `json:"key"`
		Source string `json:"source"`
	}
	var docs []entry
	for _, sv := range stageVersions {
		for _, l := range sv.lessons {
			if l.markdown != nil {
				docs = append(docs, entry{Key: l.key, Source: l.markdown.source})
			}
		}
	}
	require.Len(t, docs, 7)

	var got bytes.Buffer
	enc := json.NewEncoder(&got)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	require.NoError(t, enc.Encode(docs))

	path := filepath.Join("testdata", "seed_markdown.json")
	if *update {
		require.NoError(t, os.MkdirAll("testdata", 0o755))
		require.NoError(t, os.WriteFile(path, got.Bytes(), 0o644)) //nolint:gosec // golden JSON công khai trong repo
		return
	}
	want, err := os.ReadFile(path) //nolint:gosec // đường dẫn cố định trong testdata
	require.NoError(t, err, "chạy lại với -update")
	assert.Equal(t, string(want), got.String(), "golden seed_markdown.json khác (chạy lại với -update nếu thay đổi là chủ ý)")
}

func TestOffset(t *testing.T) {
	now := time.Date(2026, 10, 5, 12, 0, 0, 0, time.UTC)
	assert.Equal(t, now.Add(-26*time.Hour), ago(1, 2).at(now))
	assert.Equal(t, now.Add(36*time.Hour), ahead(1.5).at(now))
	assert.Equal(t, now.Add(-time.Duration(0.17*24*float64(time.Hour))), ago(0.17, 0).at(now))
}

func TestValidate(t *testing.T) {
	const pw = "Dev-Password-123"
	cases := []struct {
		name string
		cfg  config.Config
		opts Options
		want error
	}{
		{"dev", config.Config{AppEnv: config.EnvDev, SeedPassword: pw}, Options{}, nil},
		{"e2e reset", config.Config{AppEnv: config.EnvE2E, SeedPassword: pw}, Options{Reset: true}, nil},
		{"production", config.Config{AppEnv: config.EnvProduction, SeedPassword: pw}, Options{}, ErrProduction},
		{"production reset", config.Config{AppEnv: config.EnvProduction, SeedPassword: pw}, Options{Reset: true}, ErrProduction},
		{"reset outside dev and e2e", config.Config{AppEnv: "staging", SeedPassword: pw}, Options{Reset: true}, ErrResetNotAllowed},
		{"missing password", config.Config{AppEnv: config.EnvDev}, Options{}, ErrSeedPassword},
		{"short password", config.Config{AppEnv: config.EnvDev, SeedPassword: "short-pw-11"}, Options{}, ErrSeedPassword},
		{"upload sample", config.Config{AppEnv: config.EnvDev, SeedPassword: pw}, Options{UploadSample: true}, nil},
		{"upload sample production", config.Config{AppEnv: config.EnvProduction, SeedPassword: pw}, Options{UploadSample: true}, ErrProduction},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := Validate(c.cfg, c.opts)
			if c.want == nil {
				assert.NoError(t, err)
				return
			}
			assert.ErrorIs(t, err, c.want)
		})
	}
}
