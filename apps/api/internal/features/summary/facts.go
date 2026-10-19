package summary

import (
	"github.com/google/uuid"

	"lms/api/internal/features/stages"
)

type memberLesson struct{ member, lesson uuid.UUID }

// BuildFacts gộp dữ liệu hoạt động thô thành ItemFacts cho từng thành viên trong memberIDs, mỗi thành viên đủ mọi
// học liệu theo thứ tự lessons (hàm thuần). Lượt quiz trong a.Attempts phải theo attempt_no tăng dần.
func BuildFacts(lessons []Lesson, memberIDs []uuid.UUID, a Activity) map[uuid.UUID][]ItemFacts {
	progress := make(map[memberLesson]ProgressRow, len(a.Progress))
	for _, p := range a.Progress {
		progress[memberLesson{p.MemberID, p.LessonID}] = p
	}
	subs := make(map[memberLesson]SubmissionRow, len(a.Submissions))
	for _, s := range a.Submissions {
		subs[memberLesson{s.MemberID, s.LessonID}] = s
	}
	attempts := make(map[memberLesson][]AttemptRow)
	for _, at := range a.Attempts {
		k := memberLesson{at.MemberID, at.LessonID}
		attempts[k] = append(attempts[k], at)
	}
	grants := make(map[memberLesson]int, len(a.Grants))
	for _, g := range a.Grants {
		grants[memberLesson{g.MemberID, g.LessonID}] += g.Extra
	}
	out := make(map[uuid.UUID][]ItemFacts, len(memberIDs))
	for _, mid := range memberIDs {
		items := make([]ItemFacts, len(lessons))
		for i, l := range lessons {
			k := memberLesson{mid, l.ID}
			f := ItemFacts{Lesson: l}
			if p, ok := progress[k]; ok {
				f.FirstOpenedAt, f.CompletedAt = p.FirstOpenedAt, p.CompletedAt
			}
			switch l.Type {
			case stages.LessonHomework:
				h := &HomeworkFacts{}
				if due, ok := a.Deadlines[l.ID]; ok {
					h.DueAt = &due
				}
				if s, ok := subs[k]; ok {
					latest := s.Latest
					h.SubmissionCount, h.Latest = s.Count, &latest
				}
				f.Homework = h
			case stages.LessonQuiz:
				f.Quiz = quizFacts(l, attempts[k], grants[k])
			}
			items[i] = f
		}
		out[mid] = items
	}
	return out
}

// quizFacts tính lượt đã dùng, giới hạn (đánh giá: max_attempts cộng lượt cấp thêm; luyện tập: không giới hạn), lượt
// tốt nhất (tỉ lệ điểm cao nhất, so bằng nhân chéo; bằng nhau giữ lượt sớm hơn, như quizzes.Best) và lượt đã nộp gần
// nhất.
func quizFacts(l Lesson, attempts []AttemptRow, granted int) *QuizFacts {
	q := &QuizFacts{Mode: l.QuizMode, PassPercent: l.PassPercent, AttemptsUsed: len(attempts)}
	if l.QuizMode == stages.QuizAssessment && l.MaxAttempts != nil {
		n := *l.MaxAttempts + granted
		q.Limit = &n
	}
	for _, at := range attempts {
		if at.Score == nil {
			q.Open = true
			continue
		}
		q.Submitted++
		sc := Score{Score: *at.Score, Max: at.Max}
		if q.Best == nil || sc.Score*q.Best.Max > q.Best.Score*sc.Max {
			best := sc
			q.Best = &best
		}
		q.Latest = &LatestAttempt{Score: sc, ReviewStatus: at.ReviewStatus}
	}
	return q
}
