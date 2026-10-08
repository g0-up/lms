package media

import "time"

// MediaDTO là media trả cho client.
type MediaDTO struct {
	ID          string `json:"id"`
	Kind        string `json:"kind"`
	FileName    string `json:"fileName"`
	ContentType string `json:"contentType"`
	SizeBytes   int64  `json:"sizeBytes"`
	Status      string `json:"status"`
}

// UploadTicketDTO là phản hồi POST /media/uploads.
type UploadTicketDTO struct {
	MediaID   string    `json:"mediaId"`
	UploadURL string    `json:"uploadUrl"`
	ExpiresAt time.Time `json:"expiresAt"`
}

// SignedURLDTO là phản hồi GET /media/{id}/url.
type SignedURLDTO struct {
	URL       string    `json:"url"`
	ExpiresAt time.Time `json:"expiresAt"`
}

// initUploadRequest không gắn tag binding: mọi kiểm tra nằm ở entity để trả 422 với thông điệp spec.
type initUploadRequest struct {
	Kind        string `json:"kind"`
	FileName    string `json:"fileName"`
	ContentType string `json:"contentType"`
	SizeBytes   int64  `json:"sizeBytes"`
}

// ToDTO chuyển entity sang DTO.
func ToDTO(m *MediaFile) MediaDTO {
	return MediaDTO{
		ID: m.ID().String(), Kind: string(m.Kind()), FileName: m.FileName(), ContentType: m.ContentType(),
		SizeBytes: m.SizeBytes(), Status: string(m.Status()),
	}
}

func ticketDTO(t UploadTicket) UploadTicketDTO {
	return UploadTicketDTO{MediaID: t.MediaID.String(), UploadURL: t.UploadURL, ExpiresAt: t.ExpiresAt.UTC()}
}

func signedURLDTO(s SignedURL) SignedURLDTO {
	return SignedURLDTO{URL: s.URL, ExpiresAt: s.ExpiresAt.UTC()}
}
