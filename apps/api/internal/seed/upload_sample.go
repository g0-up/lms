package seed

import (
	"archive/zip"
	"bytes"
	"context"
	_ "embed"
	"fmt"
	"image"
	"image/png"
	"strings"

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

// uploadSample tải video mẫu lên mọi storage_key của media video mẫu ("seed/...") và một file mẫu cùng loại cho mỗi
// tài liệu mẫu (file kèm đề bài và file học viên nộp), rồi sửa size_bytes cho khớp object. Chạy sau khi seed commit, kể cả khi database đã seed từ trước, nên chạy lại
// an toàn (PUT ghi đè).
func uploadSample(ctx context.Context, dbx *sqlx.DB, up storage.Uploader) error {
	if err := uploadSampleVideos(ctx, dbx, up); err != nil {
		return err
	}
	return uploadSampleDocuments(ctx, dbx, up)
}

func uploadSampleVideos(ctx context.Context, dbx *sqlx.DB, up storage.Uploader) error {
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

// uploadSampleDocuments tải file mẫu cho từng tài liệu mẫu để học viên và giảng viên bấm tải được trên dev.
func uploadSampleDocuments(ctx context.Context, dbx *sqlx.DB, up storage.Uploader) error {
	var docs []struct {
		ID          string `db:"id"`
		Key         string `db:"storage_key"`
		Name        string `db:"original_name"`
		ContentType string `db:"content_type"`
	}
	if err := dbx.SelectContext(ctx, &docs, `SELECT id, storage_key, original_name, content_type FROM media_files
		WHERE kind = 'document' AND storage_key LIKE 'seed/%' ORDER BY storage_key`); err != nil {
		return fmt.Errorf("seed: đọc tài liệu mẫu: %w", err)
	}
	for _, d := range docs {
		body, err := sampleDocument(d.Name, d.ContentType)
		if err != nil {
			return err
		}
		if err := up.Put(ctx, d.Key, bytes.NewReader(body), int64(len(body)), d.ContentType); err != nil {
			return fmt.Errorf("seed: tải tài liệu mẫu %s: %w", d.Key, err)
		}
		if _, err := dbx.ExecContext(ctx, `UPDATE media_files SET size_bytes = $1 WHERE id = $2`, len(body), d.ID); err != nil {
			return fmt.Errorf("seed: cập nhật kích thước tài liệu mẫu: %w", err)
		}
	}
	return nil
}

// sampleDocument dựng nội dung mẫu đúng loại file; nội dung cố định nên chạy lại cho cùng kết quả.
func sampleDocument(name, contentType string) ([]byte, error) {
	switch {
	case contentType == "application/zip":
		return sampleZip(name)
	case contentType == "image/png":
		return samplePNG(name)
	case strings.HasPrefix(contentType, "text/plain"):
		return fmt.Appendf(nil, "# %s\n\nFile mẫu của dữ liệu seed.\n", name), nil
	default:
		return nil, fmt.Errorf("seed: chưa có file mẫu cho %s (%s)", name, contentType)
	}
}

// samplePNG dựng ảnh PNG 16x16 một màu.
func samplePNG(name string) ([]byte, error) {
	img := image.NewRGBA(image.Rect(0, 0, 16, 16))
	for i := 0; i < len(img.Pix); i += 4 {
		img.Pix[i], img.Pix[i+1], img.Pix[i+2], img.Pix[i+3] = 0x2b, 0x6c, 0xb0, 0xff
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, fmt.Errorf("seed: tạo ảnh mẫu %s: %w", name, err)
	}
	return buf.Bytes(), nil
}

// sampleZip dựng file zip mẫu chứa README.md; nội dung cố định nên chạy lại cho cùng kết quả.
func sampleZip(name string) ([]byte, error) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, err := zw.Create("README.md")
	if err == nil {
		_, err = fmt.Fprintf(w, "# %s\n\nFile kèm mẫu của dữ liệu seed: đề bài nằm trong trang homework.\n", name)
	}
	if err == nil {
		err = zw.Close()
	}
	if err != nil {
		return nil, fmt.Errorf("seed: tạo zip mẫu %s: %w", name, err)
	}
	return buf.Bytes(), nil
}

// newSampleUploader dựng client object storage từ cấu hình S3_* của tiến trình.
func newSampleUploader(cfg storage.Config) (storage.Uploader, error) {
	up, err := storage.NewMinIO(cfg)
	if err != nil {
		return nil, fmt.Errorf("seed: %w", err)
	}
	return up, nil
}
