package media

import (
	"context"

	"github.com/google/uuid"
)

// URLSigner là cổng ký URL xem cho feature khác (learning dựng nội dung học liệu video); cùng kiểm quyền với
// GET /media/{id}/url.
type URLSigner interface {
	SignedURL(ctx context.Context, user Principal, id uuid.UUID) (SignedURL, error)
}

var _ URLSigner = (*Service)(nil)
