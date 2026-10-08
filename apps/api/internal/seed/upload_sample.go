package seed

import (
	"bytes"
	"context"
	_ "embed"
	"fmt"

	"github.com/jmoiron/sqlx"

	"lms/api/internal/platform/storage"
)

// sampleVideo là video H.264/AAC 2 giây (320x180, faststart) tạo bằng ffmpeg; mọi bài video mẫu dùng chung file này
// để trình phát tua được trên dev mà không cần video thật.
//
//go:embed assets/sample.mp4
var sampleVideo []byte

// sampleVideoContentType khớp content_type của media_files video mẫu.
const sampleVideoContentType = "video/mp4"

// uploadSample tải video mẫu lên mọi storage_key của media video mẫu ("seed/...") rồi sửa size_bytes cho khớp object.
// Chạy sau khi seed commit, kể cả khi database đã seed từ trước, nên chạy lại an toàn (PUT ghi đè).
func uploadSample(ctx context.Context, dbx *sqlx.DB, up storage.Uploader) error {
	var keys []string
	if err := dbx.SelectContext(ctx, &keys,
		`SELECT storage_key FROM media_files WHERE kind = 'video' AND storage_key LIKE 'seed/%' ORDER BY storage_key`); err != nil {
		return fmt.Errorf("seed: đọc media mẫu: %w", err)
	}
	size := int64(len(sampleVideo))
	for _, key := range keys {
		if err := up.Put(ctx, key, bytes.NewReader(sampleVideo), size, sampleVideoContentType); err != nil {
			return fmt.Errorf("seed: tải video mẫu %s: %w", key, err)
		}
	}
	if _, err := dbx.ExecContext(ctx,
		`UPDATE media_files SET size_bytes = $1 WHERE kind = 'video' AND storage_key LIKE 'seed/%'`, size); err != nil {
		return fmt.Errorf("seed: cập nhật kích thước video mẫu: %w", err)
	}
	return nil
}

// newSampleUploader dựng client object storage từ cấu hình S3_* của tiến trình.
func newSampleUploader(cfg storage.Config) (storage.Uploader, error) {
	up, err := storage.NewMinIO(cfg)
	if err != nil {
		return nil, fmt.Errorf("seed: %w", err)
	}
	return up, nil
}
