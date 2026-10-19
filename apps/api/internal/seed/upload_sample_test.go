//go:build integration

package seed

import (
	"archive/zip"
	"bytes"
	"image/png"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"lms/api/internal/platform/storage"
)

func TestUploadSamplePutsEveryVideoAndFixesSize(t *testing.T) {
	dbx := openEmpty(t)
	ctx := testContext(t)
	_, err := Run(ctx, dbx, testConfig(t), fixedClock{time.Now()}, Options{})
	require.NoError(t, err)

	store := storage.NewMemStorage()
	require.NoError(t, uploadSample(ctx, dbx, store))
	require.NoError(t, uploadSample(ctx, dbx, store), "chạy lại ghi đè, không lỗi")

	var rows []struct {
		Key  string `db:"storage_key"`
		Size int64  `db:"size_bytes"`
	}
	require.NoError(t, dbx.SelectContext(ctx, &rows, `SELECT storage_key, size_bytes FROM media_files WHERE kind = 'video'`))
	require.Len(t, rows, videoLessonCount())
	for _, r := range rows {
		st, err := store.Stat(ctx, r.Key)
		require.NoError(t, err, r.Key)
		assert.Equal(t, int64(len(sampleVideo)), st.Size, r.Key)
		assert.Equal(t, "video/mp4", st.ContentType, r.Key)
		assert.Equal(t, st.Size, r.Size, "size_bytes khớp object: %s", r.Key)
	}

	var docs []struct {
		Key         string `db:"storage_key"`
		Name        string `db:"original_name"`
		ContentType string `db:"content_type"`
		Size        int64  `db:"size_bytes"`
	}
	require.NoError(t, dbx.SelectContext(ctx, &docs,
		`SELECT storage_key, original_name, content_type, size_bytes FROM media_files WHERE kind = 'document'`))
	require.Len(t, docs, 5, "một file kèm đề bài và bốn file học viên nộp")
	types := map[string]int{}
	for _, d := range docs {
		types[d.ContentType]++
		st, err := store.Stat(ctx, d.Key)
		require.NoError(t, err, d.Key)
		assert.Equal(t, d.ContentType, st.ContentType, d.Key)
		assert.Equal(t, st.Size, d.Size, "size_bytes khớp object: %s", d.Key)
		want, err := sampleDocument(d.Name, d.ContentType)
		require.NoError(t, err)
		assert.Equal(t, int64(len(want)), st.Size, d.Key)
	}
	assert.Equal(t, map[string]int{"application/zip": 3, "image/png": 1, "text/plain; charset=utf-8": 1}, types)
}

// TestSampleDocumentMatchesType: file mẫu đúng định dạng của content type; loại chưa có mẫu báo lỗi rõ.
func TestSampleDocumentMatchesType(t *testing.T) {
	img, err := sampleDocument("ket-qua-chay.png", "image/png")
	require.NoError(t, err)
	cfg, err := png.DecodeConfig(bytes.NewReader(img))
	require.NoError(t, err)
	assert.Equal(t, 16, cfg.Width)

	text, err := sampleDocument("README.md", "text/plain; charset=utf-8")
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(string(text), "# README.md\n"))

	_, err = sampleDocument("de-bai.pdf", "application/pdf")
	require.ErrorContains(t, err, "chưa có file mẫu cho de-bai.pdf")
}

// TestSampleZipIsValid: file kèm mẫu là zip mở được, chứa README nêu tên file, và dựng lại cho cùng nội dung.
func TestSampleZipIsValid(t *testing.T) {
	body, err := sampleZip("ds-stack-queue-starter.zip")
	require.NoError(t, err)
	again, err := sampleZip("ds-stack-queue-starter.zip")
	require.NoError(t, err)
	assert.Equal(t, body, again)

	zr, err := zip.NewReader(bytes.NewReader(body), int64(len(body)))
	require.NoError(t, err)
	require.Len(t, zr.File, 1)
	assert.Equal(t, "README.md", zr.File[0].Name)
	assert.Contains(t, string(readZipFile(t, zr.File[0])), "# ds-stack-queue-starter.zip")
}

func readZipFile(t *testing.T, f *zip.File) []byte {
	t.Helper()
	rc, err := f.Open()
	require.NoError(t, err)
	defer func() { _ = rc.Close() }()
	b, err := io.ReadAll(rc)
	require.NoError(t, err)
	return b
}

func TestSampleVideoIsMP4(t *testing.T) {
	require.Greater(t, len(sampleVideo), 12)
	assert.Equal(t, "ftyp", string(sampleVideo[4:8]), "file mẫu phải là MP4 (ISO BMFF)")
}
