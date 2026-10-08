package httpx

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"lms/api/internal/platform/apperr"
	"lms/api/internal/platform/ids"
)

// UUIDParam đọc path param name dạng uuid chuẩn. Sai định dạng trả 404 NOT_FOUND giống id không tồn tại,
// để không gợi ý id nào hợp lệ.
func UUIDParam(c *gin.Context, name string) (uuid.UUID, error) {
	id, err := ids.Parse(c.Param(name))
	if err != nil {
		return uuid.Nil, apperr.NotFound("dữ liệu")
	}
	return id, nil
}
