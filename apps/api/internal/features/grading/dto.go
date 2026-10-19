package grading

import "time"

// PersonDTO là học viên hoặc người chấm.
type PersonDTO struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// ClassDTO là lớp của một mục.
type ClassDTO struct {
	ID   string `json:"id"`
	Code string `json:"code"`
	Name string `json:"name"`
}

// LessonDTO là học liệu của một mục.
type LessonDTO struct {
	ID    string `json:"id"`
	Title string `json:"title"`
}

// StageDTO là chặng chứa học liệu.
type StageDTO struct {
	Code string `json:"code"`
	Name string `json:"name"`
}

// QueueItemDTO là một mục chờ xử lý. scorePercent chỉ có ở quiz (% tổng của lượt, không có đáp án hay điểm từng câu),
// homework là null.
type QueueItemDTO struct {
	Kind          string    `json:"kind"`
	ItemID        string    `json:"itemId"`
	SubmittedAt   time.Time `json:"submittedAt"`
	No            int       `json:"no"`
	IsLate        bool      `json:"isLate"`
	AutoSubmitted bool      `json:"autoSubmitted"`
	ScorePercent  *int      `json:"scorePercent"`
	Student       PersonDTO `json:"student"`
	Class         ClassDTO  `json:"class"`
	Lesson        LessonDTO `json:"lesson"`
	Stage         StageDTO  `json:"stage"`
	WaitedHours   int       `json:"waitedHours"`
	Overdue       bool      `json:"overdue"`
}

// ToQueueItemDTO là dạng JSON của một mục; homework và quizzes dùng cho nextQueueItem.
func ToQueueItemDTO(q QueueItem) QueueItemDTO {
	return QueueItemDTO{
		Kind: string(q.Key.Kind), ItemID: q.Key.ItemID.String(), SubmittedAt: q.SubmittedAt.UTC(), No: q.No,
		IsLate: q.IsLate, AutoSubmitted: q.AutoSubmitted, ScorePercent: q.ScorePercent(),
		Student:     PersonDTO{ID: q.Student.ID.String(), Name: q.Student.Name},
		Class:       toClassDTO(q.Class),
		Lesson:      LessonDTO{ID: q.Lesson.ID.String(), Title: q.Lesson.Name},
		Stage:       StageDTO{Code: q.Stage.Code, Name: q.Stage.Name},
		WaitedHours: q.WaitedHours, Overdue: q.Overdue,
	}
}

func toClassDTO(c Ref) ClassDTO {
	return ClassDTO{ID: c.ID.String(), Code: c.Code, Name: c.Name}
}

// LessonOptionDTO là một option của ô lọc học liệu.
type LessonOptionDTO struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	Kind  string `json:"kind"`
}

// FiltersDTO là các option lọc, chỉ gồm lớp và học liệu actor thấy.
type FiltersDTO struct {
	Classes []ClassDTO        `json:"classes"`
	Lessons []LessonOptionDTO `json:"lessons"`
}

// QueueDTO là phản hồi GET /grading/queue. total, quizPending và overdueCount đếm trên toàn hàng chờ của actor, không
// theo bộ lọc.
type QueueDTO struct {
	Items        []QueueItemDTO `json:"items"`
	Total        int            `json:"total"`
	QuizPending  int            `json:"quizPending"`
	OverdueCount int            `json:"overdueCount"`
	Filters      FiltersDTO     `json:"filters"`
}

func toQueueDTO(q Queue) QueueDTO {
	items := make([]QueueItemDTO, len(q.Items))
	for i, it := range q.Items {
		items[i] = ToQueueItemDTO(it)
	}
	classes := make([]ClassDTO, len(q.Classes))
	for i, c := range q.Classes {
		classes[i] = toClassDTO(c)
	}
	lessons := make([]LessonOptionDTO, len(q.Lessons))
	for i, l := range q.Lessons {
		lessons[i] = LessonOptionDTO{ID: l.ID.String(), Title: l.Title, Kind: string(l.Kind)}
	}
	return QueueDTO{
		Items: items, Total: q.Total, QuizPending: q.QuizPending, OverdueCount: q.OverdueCount,
		Filters: FiltersDTO{Classes: classes, Lessons: lessons},
	}
}

// RecentDTO là một lần chấm homework hoặc xác nhận quiz bài tập. points/maxPoints là điểm bài hoặc điểm lượt quiz.
type RecentDTO struct {
	Kind      string    `json:"kind"`
	ItemID    string    `json:"itemId"`
	At        time.Time `json:"at"`
	By        PersonDTO `json:"by"`
	Student   PersonDTO `json:"student"`
	Class     ClassDTO  `json:"class"`
	Lesson    LessonDTO `json:"lesson"`
	No        int       `json:"no"`
	Points    int       `json:"points"`
	MaxPoints int       `json:"maxPoints"`
	Passed    bool      `json:"passed"`
}

// RecentListDTO là phản hồi GET /grading/recent.
type RecentListDTO struct {
	Items []RecentDTO `json:"items"`
}

func toRecentListDTO(rs []Recent) RecentListDTO {
	items := make([]RecentDTO, len(rs))
	for i, r := range rs {
		items[i] = RecentDTO{
			Kind: string(r.Key.Kind), ItemID: r.Key.ItemID.String(), At: r.At.UTC(),
			By:      PersonDTO{ID: r.By.ID.String(), Name: r.By.Name},
			Student: PersonDTO{ID: r.Student.ID.String(), Name: r.Student.Name},
			Class:   toClassDTO(r.Class), Lesson: LessonDTO{ID: r.Lesson.ID.String(), Title: r.Lesson.Name},
			No: r.No, Points: r.Points, MaxPoints: r.MaxPoints, Passed: r.Passed,
		}
	}
	return RecentListDTO{Items: items}
}
