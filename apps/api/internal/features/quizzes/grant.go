package quizzes

import (
	"fmt"
	"html"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/microcosm-cc/bluemonday"

	"lms/api/internal/domain"
)

const (
	minGrantAttempts = 1
	maxGrantAttempts = 3
	maxReasonRunes   = 500
	// questionTextRunes là độ dài tối đa của bản văn bản thuần câu hỏi trong bảng thống kê.
	questionTextRunes = 120
)

// Lỗi của cấp thêm lượt; copy theo prototype/app.js (grantAttempts).
var (
	ErrNotOwnGrant         = domain.ErrForbidden.WithMsg("Bạn chỉ cấp thêm lượt cho lớp mình phụ trách.")
	ErrGrantClassNotActive = domain.ErrInvalidTransition.WithMsg("Lớp không ở trạng thái đang chạy")
	ErrGrantNotAssessment  = domain.ErrInvalid.WithMsg("Chỉ cấp thêm lượt cho quiz bài tập.")
	ErrGrantAttemptsRange  = domain.ErrInvalid.WithMsg(fmt.Sprintf("Số lượt cấp thêm từ %d đến %d.", minGrantAttempts, maxGrantAttempts))
	ErrGrantReasonRequired = domain.ErrInvalid.WithMsg("Nhập lý do cấp thêm lượt. Lý do được lưu vào lịch sử.")
	ErrGrantReasonTooLong  = domain.ErrInvalid.WithMsg(fmt.Sprintf("Lý do tối đa %d ký tự.", maxReasonRunes))
)

// GrantInput là yêu cầu cấp thêm lượt; ExtraAttempts nil là thiếu trường.
type GrantInput struct {
	LessonID      uuid.UUID
	ExtraAttempts *int
	Reason        string
}

// grantContent kiểm số lượt và lý do (hàm thuần), trả số lượt cùng lý do đã trim.
func grantContent(in GrantInput) (int, string, error) {
	if in.ExtraAttempts == nil || *in.ExtraAttempts < minGrantAttempts || *in.ExtraAttempts > maxGrantAttempts {
		return 0, "", ErrGrantAttemptsRange
	}
	reason := strings.TrimSpace(in.Reason)
	switch {
	case reason == "":
		return 0, "", ErrGrantReasonRequired
	case utf8.RuneCountInString(reason) > maxReasonRunes:
		return 0, "", ErrGrantReasonTooLong
	}
	return *in.ExtraAttempts, reason, nil
}

// QuestionStats là tỉ lệ trả lời đúng từng câu của một quiz; Attempts là số lượt đã nộp làm mẫu số.
type QuestionStats struct {
	Attempts int
	Rows     []QuestionStat
}

// QuestionStat là một câu trong thống kê: Position đánh số câu từ 1 theo thứ tự soạn, CorrectPercent nil khi quiz
// chưa có lượt nào.
type QuestionStat struct {
	QuestionID     uuid.UUID
	Position       int
	BodyText       string
	CorrectPercent *int
}

// questionStats dựng thống kê câu hỏi của một quiz từ số câu đúng và số lượt đã nộp (hàm thuần).
func questionStats(rows []QuestionTally, attempts int) QuestionStats {
	out := QuestionStats{Attempts: attempts, Rows: make([]QuestionStat, len(rows))}
	for i, r := range rows {
		st := QuestionStat{QuestionID: r.QuestionID, Position: i + 1, BodyText: questionText(r.BodyHTML, r.BodySource)}
		if attempts > 0 {
			p := domain.Percent(r.Correct, attempts)
			st.CorrectPercent = &p
		}
		out.Rows[i] = st
	}
	return out
}

// plainText bỏ mọi thẻ HTML; dùng chung vì Policy an toàn khi gọi đồng thời.
var plainText = bluemonday.StrictPolicy()

// blockTag là thẻ khối; thêm khoảng trắng trước thẻ để chữ của hai khối liền nhau không dính vào nhau khi bỏ thẻ.
var blockTag = regexp.MustCompile(`(?i)<(/?(p|div|li|ul|ol|h[1-6]|pre|blockquote|table|tr|td|th)\b|br\b)`)

// questionText là bản văn bản thuần một dòng của câu hỏi (ưu tiên HTML đã render, không có thì nguồn), cắt ở
// questionTextRunes ký tự kể cả dấu "…".
func questionText(bodyHTML *string, source string) string {
	text := source
	if bodyHTML != nil {
		text = html.UnescapeString(plainText.Sanitize(blockTag.ReplaceAllString(*bodyHTML, " <$1")))
	}
	text = strings.Join(strings.Fields(text), " ")
	if utf8.RuneCountInString(text) <= questionTextRunes {
		return text
	}
	r := []rune(text)
	return strings.TrimRight(string(r[:questionTextRunes-1]), " ") + "…"
}
