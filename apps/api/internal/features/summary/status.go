package summary

import (
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"lms/api/internal/domain"
	"lms/api/internal/features/stages"
)

// Status là trạng thái hiển thị của một học liệu (hàm thuần). Done chỉ lấy từ completed_at, là nguồn duy nhất của
// hoàn thành nên khớp % của learning và reports; nhãn theo lessonStatus của prototype, chi tiết theo FR-80.
func Status(f ItemFacts, now time.Time) ItemStatus {
	done := f.CompletedAt != nil
	switch f.Lesson.Type {
	case stages.LessonQuiz:
		q := QuizFacts{Mode: f.Lesson.QuizMode}
		if f.Quiz != nil {
			q = *f.Quiz
		}
		if q.Mode == stages.QuizPractice {
			return practiceStatus(q, done)
		}
		return assessmentStatus(q, done)
	case stages.LessonHomework:
		h := HomeworkFacts{}
		if f.Homework != nil {
			h = *f.Homework
		}
		return homeworkStatus(h, done, now)
	}
	return contentStatus(f, done)
}

// contentStatus: video và markdown do học viên tự tích.
func contentStatus(f ItemFacts, done bool) ItemStatus {
	switch {
	case done:
		return ItemStatus{Done: true, Label: "Đã học xong", Tone: ToneOK, Detail: "Đã tích hoàn thành"}
	case f.FirstOpenedAt != nil:
		return ItemStatus{Label: "Chưa học", Tone: ToneOptional, Detail: "Đã mở, chưa tích hoàn thành"}
	}
	return ItemStatus{Label: "Chưa học", Tone: ToneOptional, Detail: "Chưa mở"}
}

// practiceStatus: quiz học liệu hoàn thành khi nộp ít nhất một lượt; chi tiết là số lượt đã nộp và điểm lượt gần
// nhất.
func practiceStatus(q QuizFacts, done bool) ItemStatus {
	detail := "Chưa làm"
	if q.Submitted > 0 && q.Latest != nil {
		detail = fmt.Sprintf("%d lượt · Gần nhất %d%%", q.Submitted, q.Latest.Percent())
	}
	switch {
	case done:
		return ItemStatus{Done: true, Label: "Đã làm", Tone: ToneOK, Detail: detail}
	case q.Open:
		return ItemStatus{Label: "Đang làm", Tone: ToneInfo, Detail: detail}
	}
	return ItemStatus{Label: "Chưa làm", Tone: ToneOptional, Detail: detail}
}

// assessmentStatus: quiz bài tập hoàn thành khi lượt tốt nhất đạt ngưỡng; chưa đạt mà hết lượt (kể cả lượt cấp
// thêm) là nguy cấp. Chi tiết là điểm tốt nhất, lượt đã dùng trên tổng và trạng thái xác nhận của lượt gần nhất.
func assessmentStatus(q QuizFacts, done bool) ItemStatus {
	st := ItemStatus{Detail: assessmentDetail(q)}
	switch {
	case done && q.Best != nil:
		st.Done, st.Label, st.Tone = true, fmt.Sprintf("Đạt %d%%", q.Best.Percent()), ToneOK
	case done:
		st.Done, st.Label, st.Tone = true, "Đạt", ToneOK
	case q.Open:
		st.Label, st.Tone = "Đang làm", ToneInfo
	case q.Best != nil && q.Limit != nil && q.AttemptsUsed >= *q.Limit:
		st.Label, st.Tone = fmt.Sprintf("Chưa đạt %d%% · hết lượt", q.Best.Percent()), ToneDanger
	case q.Best != nil:
		st.Label, st.Tone = fmt.Sprintf("Chưa đạt %d%%", q.Best.Percent()), ToneWarn
	default:
		st.Label, st.Tone = "Chưa làm", ToneOptional
	}
	return st
}

func assessmentDetail(q QuizFacts) string {
	used := fmt.Sprintf("%d lượt", q.AttemptsUsed)
	if q.Limit != nil {
		used = fmt.Sprintf("%d/%d lượt", q.AttemptsUsed, *q.Limit)
	}
	if q.Best == nil {
		if q.AttemptsUsed == 0 && q.Limit != nil {
			return fmt.Sprintf("Chưa làm · %d lượt", *q.Limit)
		}
		if q.AttemptsUsed == 0 {
			return "Chưa làm"
		}
		return used
	}
	parts := []string{fmt.Sprintf("Tốt nhất %d%%", q.Best.Percent()), used}
	if q.Latest != nil && q.Latest.ReviewStatus != nil {
		if l := q.Latest.ReviewStatus.Label(); l != "" {
			parts = append(parts, l)
		}
	}
	return strings.Join(parts, " · ")
}

// homeworkStatus: nhãn theo bản nộp mới nhất. "Quá hạn" suy ra khi có hạn, now > dueAt và chưa nộp lần nào (đúng
// bằng hạn chưa quá hạn); không lưu ở đâu. Chi tiết là số lần nộp, điểm lần chấm hiện hành và "nộp muộn".
func homeworkStatus(h HomeworkFacts, done bool, now time.Time) ItemStatus {
	st := ItemStatus{Done: done}
	if h.Latest == nil {
		st.Detail = "Chưa nộp"
		if h.DueAt == nil {
			st.Detail = "Chưa nộp · chưa đặt hạn"
		}
		if h.DueAt != nil && now.After(*h.DueAt) {
			st.Label, st.Tone, st.Overdue = "Quá hạn", ToneDanger, true
			return st
		}
		st.Label, st.Tone = "Chưa nộp", ToneOptional
		return st
	}
	parts := []string{fmt.Sprintf("%d lần nộp", h.SubmissionCount)}
	if h.Latest.Total != nil && h.Latest.Max != nil {
		parts = append(parts, fmt.Sprintf("%d/%d điểm", *h.Latest.Total, *h.Latest.Max))
	}
	if h.Latest.IsLate {
		parts = append(parts, "nộp muộn")
	}
	st.Detail = strings.Join(parts, " · ")
	switch h.Latest.Status {
	case SubmissionPassed:
		st.Label, st.Tone = "Đạt", ToneOK
	case SubmissionRework:
		st.Label, st.Tone, st.Rework = "Cần làm lại", ToneWarn, true
	default:
		st.Label, st.Tone = "Chờ chấm", ToneInfo
	}
	return st
}

// BuildStage dựng tổng kết một chặng từ dữ liệu thô của các học liệu trong chặng (theo thứ tự bài học → học liệu).
// Bài học nhóm theo UnitID liên tiếp; Missing là học liệu bắt buộc chưa hoàn thành; cờ Rework/Overdue bật khi có
// bất kỳ học liệu nào mang cờ đó.
func BuildStage(stage StageRef, facts []ItemFacts, review *Review, now time.Time) StageSummary {
	out := StageSummary{Stage: stage, Units: []UnitItems{}, Missing: []MissingItem{}, Review: review}
	for _, f := range facts {
		st := Status(f, now)
		if len(out.Units) == 0 || out.Units[len(out.Units)-1].ID != f.Lesson.UnitID {
			out.Units = append(out.Units, UnitItems{
				ID: f.Lesson.UnitID, Title: f.Lesson.UnitTitle, Position: f.Lesson.UnitPosition, Items: []Item{},
			})
		}
		u := &out.Units[len(out.Units)-1]
		u.Items = append(u.Items, Item{Facts: f, Status: st})
		if f.Lesson.Required {
			out.Required++
			if st.Done {
				out.Done++
			} else {
				out.Missing = append(out.Missing, MissingItem{LessonID: f.Lesson.ID, Title: f.Lesson.Title, UnitTitle: f.Lesson.UnitTitle})
			}
		}
		out.Rework = out.Rework || st.Rework
		out.Overdue = out.Overdue || st.Overdue
	}
	out.Percent = domain.Percent(out.Done, out.Required)
	out.Complete = out.Required > 0 && out.Done == out.Required
	return out
}

// BuildCourse dựng tổng kết mọi chặng của một thành viên; facts là dữ liệu thô của mọi học liệu trong phiên bản
// khóa học, reviews là nhận xét theo phiên bản chặng. Trả các chặng cùng số bắt buộc đã xong và tổng của toàn khóa.
func BuildCourse(stageRefs []StageRef, facts []ItemFacts, reviews map[uuid.UUID]*Review, now time.Time) ([]StageSummary, int, int) {
	byStage := make(map[uuid.UUID][]ItemFacts, len(stageRefs))
	for _, f := range facts {
		byStage[f.Lesson.StageVersionID] = append(byStage[f.Lesson.StageVersionID], f)
	}
	out := make([]StageSummary, len(stageRefs))
	done, total := 0, 0
	for i, s := range stageRefs {
		out[i] = BuildStage(s, byStage[s.StageVersionID], reviews[s.StageVersionID], now)
		done += out[i].Done
		total += out[i].Required
	}
	return out, done, total
}
