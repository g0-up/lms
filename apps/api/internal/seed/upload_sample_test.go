//go:build integration

package seed

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"lms/api/internal/platform/storage"
	"lms/api/internal/platform/testdb"
)

func TestUploadSamplePutsEveryVideoAndFixesSize(t *testing.T) {
	dbx := testdb.Open(t)
	testdb.Reset(t, dbx)
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
}

func TestSampleVideoIsMP4(t *testing.T) {
	require.Greater(t, len(sampleVideo), 12)
	assert.Equal(t, "ftyp", string(sampleVideo[4:8]), "file mẫu phải là MP4 (ISO BMFF)")
}
