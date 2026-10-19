package stages

import (
	"context"
	"fmt"
	"slices"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"lms/api/internal/domain"
	"lms/api/internal/platform/db"
	"lms/api/internal/platform/db/pgerr"
)

// Nội dung quiz và homework nằm ở bảng con của lessons. Như các câu ghi lessons, mọi câu ghi bảng con mang điều kiện
// header còn draft ngay trong SQL; câu cần đúng một dòng chạy qua db.ExecAffectOne để 0 dòng thành ErrVersionImmutable.

// draftLessonOfT: dòng t (bảng con có cột lesson_id) thuộc học liệu của bản nháp.
const draftLessonOfT = `EXISTS (SELECT 1 FROM lessons dl JOIN stage_versions dsv ON dsv.id = dl.stage_version_id
	WHERE dl.id = t.lesson_id AND dsv.status = 'draft')`

// draftQuestionOfT: phương án t thuộc câu hỏi của bản nháp.
const draftQuestionOfT = `EXISTS (SELECT 1 FROM questions dq JOIN lessons dl ON dl.id = dq.lesson_id
	JOIN stage_versions dsv ON dsv.id = dl.stage_version_id WHERE dq.id = t.question_id AND dsv.status = 'draft')`

// fromDraftLesson là nguồn của INSERT … SELECT vào bảng con: chỉ ra một dòng khi học liệu $1 thuộc bản nháp.
const fromDraftLesson = ` FROM lessons l JOIN stage_versions sv ON sv.id = l.stage_version_id WHERE l.id = $1 AND sv.status = 'draft'`

type quizSettingsRecord struct {
	LessonID         uuid.UUID `db:"lesson_id"`
	Mode             string    `db:"mode"`
	MaxAttempts      *int      `db:"max_attempts"`
	TimeLimitSeconds *int      `db:"time_limit_seconds"`
	Shuffle          bool      `db:"shuffle"`
	PassPercent      *int      `db:"pass_percent"`
}

type questionRecord struct {
	ID                uuid.UUID `db:"id"`
	LessonID          uuid.UUID `db:"lesson_id"`
	QuestionKey       string    `db:"question_key"`
	Type              string    `db:"type"`
	BodySource        string    `db:"body_source"`
	BodyHTML          *string   `db:"body_html"`
	ExplanationSource string    `db:"explanation_source"`
	ExplanationHTML   *string   `db:"explanation_html"`
	Points            int       `db:"points"`
}

type optionRecord struct {
	ID         uuid.UUID `db:"id"`
	QuestionID uuid.UUID `db:"question_id"`
	Body       string    `db:"body"`
	IsCorrect  bool      `db:"is_correct"`
}

type homeworkSettingsRecord struct {
	LessonID    uuid.UUID `db:"lesson_id"`
	BriefSource string    `db:"brief_source"`
	BriefHTML   *string   `db:"brief_html"`
	PassPercent int       `db:"pass_percent"`
}

type criterionRecord struct {
	ID          uuid.UUID `db:"id"`
	LessonID    uuid.UUID `db:"lesson_id"`
	Description string    `db:"description"`
	MaxPoints   int       `db:"max_points"`
}

type attachmentRecord struct {
	LessonID  uuid.UUID `db:"lesson_id"`
	MediaID   uuid.UUID `db:"media_id"`
	FileName  string    `db:"file_name"`
	SizeBytes int64     `db:"size_bytes"`
}

// lessonContents là nội dung quiz và homework của cả phiên bản theo id học liệu.
type lessonContents struct {
	quiz     map[uuid.UUID]QuizContent
	homework map[uuid.UUID]HomeworkContent
}

// loadContents đọc theo lô nội dung quiz/homework của phiên bản: một truy vấn mỗi bảng con, chỉ khi phiên bản có
// loại học liệu đó.
func loadContents(ctx context.Context, ex db.Executor, versionID uuid.UUID, recs []lessonRecord) (lessonContents, error) {
	out := lessonContents{quiz: map[uuid.UUID]QuizContent{}, homework: map[uuid.UUID]HomeworkContent{}}
	hasType := func(t LessonType) bool {
		return slices.ContainsFunc(recs, func(r lessonRecord) bool { return LessonType(r.Type) == t })
	}
	if hasType(LessonQuiz) {
		if err := loadQuizzes(ctx, ex, versionID, out.quiz); err != nil {
			return out, err
		}
	}
	if hasType(LessonHomework) {
		if err := loadHomeworks(ctx, ex, versionID, out.homework); err != nil {
			return out, err
		}
	}
	return out, nil
}

func loadQuizzes(ctx context.Context, ex db.Executor, versionID uuid.UUID, into map[uuid.UUID]QuizContent) error {
	var settings []quizSettingsRecord
	if err := sqlx.SelectContext(ctx, ex, &settings, `SELECT t.lesson_id, t.mode, t.max_attempts, t.time_limit_seconds,
		t.shuffle, t.pass_percent FROM quiz_settings t JOIN lessons l ON l.id = t.lesson_id
		WHERE l.stage_version_id = $1`, versionID); err != nil {
		return fmt.Errorf("stages: đọc cấu hình quiz: %w", err)
	}
	var questions []questionRecord
	if err := sqlx.SelectContext(ctx, ex, &questions, `SELECT q.id, q.lesson_id, q.question_key, q.type, q.body_source,
		q.body_html, q.explanation_source, q.explanation_html, q.points
		FROM questions q JOIN lessons l ON l.id = q.lesson_id
		WHERE l.stage_version_id = $1 ORDER BY q.lesson_id, q.position`, versionID); err != nil {
		return fmt.Errorf("stages: đọc câu hỏi: %w", err)
	}
	var options []optionRecord
	if err := sqlx.SelectContext(ctx, ex, &options, `SELECT o.id, o.question_id, o.body, o.is_correct
		FROM question_options o JOIN questions q ON q.id = o.question_id JOIN lessons l ON l.id = q.lesson_id
		WHERE l.stage_version_id = $1 ORDER BY o.question_id, o.position`, versionID); err != nil {
		return fmt.Errorf("stages: đọc phương án: %w", err)
	}
	optionsOf := make(map[uuid.UUID][]Option, len(questions))
	for _, o := range options {
		optionsOf[o.QuestionID] = append(optionsOf[o.QuestionID], Option{ID: o.ID, Body: o.Body, Correct: o.IsCorrect})
	}
	questionsOf := make(map[uuid.UUID][]*Question, len(settings))
	for _, q := range questions {
		questionsOf[q.LessonID] = append(questionsOf[q.LessonID], &Question{
			ID: q.ID, Key: domain.LessonKey(q.QuestionKey), Type: QuestionType(q.Type), BodySource: q.BodySource,
			BodyHTML: q.BodyHTML, ExplanationSource: q.ExplanationSource, ExplanationHTML: q.ExplanationHTML,
			Points: q.Points, Options: optionsOf[q.ID],
		})
	}
	for _, s := range settings {
		c := QuizContent{Mode: QuizMode(s.Mode), MaxAttempts: s.MaxAttempts, Shuffle: s.Shuffle, PassPercent: s.PassPercent,
			Questions: questionsOf[s.LessonID]}
		if s.TimeLimitSeconds != nil {
			d := time.Duration(*s.TimeLimitSeconds) * time.Second
			c.TimeLimit = &d
		}
		into[s.LessonID] = c
	}
	return nil
}

func loadHomeworks(ctx context.Context, ex db.Executor, versionID uuid.UUID, into map[uuid.UUID]HomeworkContent) error {
	var settings []homeworkSettingsRecord
	if err := sqlx.SelectContext(ctx, ex, &settings, `SELECT t.lesson_id, t.brief_source, t.brief_html, t.pass_percent
		FROM homework_settings t JOIN lessons l ON l.id = t.lesson_id WHERE l.stage_version_id = $1`, versionID); err != nil {
		return fmt.Errorf("stages: đọc đề homework: %w", err)
	}
	var criteria []criterionRecord
	if err := sqlx.SelectContext(ctx, ex, &criteria, `SELECT c.id, c.lesson_id, c.description, c.max_points
		FROM rubric_criteria c JOIN lessons l ON l.id = c.lesson_id
		WHERE l.stage_version_id = $1 ORDER BY c.lesson_id, c.position`, versionID); err != nil {
		return fmt.Errorf("stages: đọc tiêu chí chấm: %w", err)
	}
	var attachments []attachmentRecord
	if err := sqlx.SelectContext(ctx, ex, &attachments, `SELECT a.lesson_id, a.media_id, mf.original_name AS file_name,
		mf.size_bytes FROM homework_attachments a JOIN lessons l ON l.id = a.lesson_id JOIN media_files mf ON mf.id = a.media_id
		WHERE l.stage_version_id = $1 ORDER BY a.lesson_id, a.position`, versionID); err != nil {
		return fmt.Errorf("stages: đọc file kèm: %w", err)
	}
	criteriaOf := make(map[uuid.UUID][]*Criterion, len(settings))
	for _, c := range criteria {
		criteriaOf[c.LessonID] = append(criteriaOf[c.LessonID], &Criterion{ID: c.ID, Description: c.Description, MaxPoints: c.MaxPoints})
	}
	attachmentsOf := make(map[uuid.UUID][]Attachment, len(settings))
	for _, a := range attachments {
		attachmentsOf[a.LessonID] = append(attachmentsOf[a.LessonID], Attachment{MediaID: a.MediaID, FileName: a.FileName, SizeBytes: a.SizeBytes})
	}
	for _, s := range settings {
		into[s.LessonID] = HomeworkContent{BriefSource: s.BriefSource, BriefHTML: s.BriefHTML, PassPercent: s.PassPercent,
			Criteria: criteriaOf[s.LessonID], Attachments: attachmentsOf[s.LessonID]}
	}
	return nil
}

// lessonMediaIDs là các media học liệu tham chiếu (không trùng, giữ thứ tự): video, ảnh nội bộ trong mọi HTML đã
// render và file kèm homework. Đây là nguồn của lesson_media, nên học viên và giảng viên của lớp tải được file kèm.
func lessonMediaIDs(c LessonContent) []uuid.UUID {
	var out []uuid.UUID
	add := func(id uuid.UUID) {
		if !slices.Contains(out, id) {
			out = append(out, id)
		}
	}
	if vc, ok := c.(VideoContent); ok {
		add(vc.MediaID)
	}
	for _, html := range renderedHTML(c) {
		for _, id := range mediaIDsInHTML(html) {
			add(id)
		}
	}
	if hc, ok := c.(HomeworkContent); ok {
		for _, a := range hc.Attachments {
			add(a.MediaID)
		}
	}
	return out
}

// saveContent ghi bảng con theo nội dung của l; old là nội dung đang lưu (nil khi học liệu mới). Đổi sang loại khác
// thì xóa bảng con của loại cũ; đổi giữa luyện tập và đánh giá vẫn là quiz nên câu hỏi được diff như thường.
func saveContent(ctx context.Context, ex db.Executor, l *Lesson, old LessonContent) error {
	oldQuiz, wasQuiz := old.(QuizContent)
	oldHomework, wasHomework := old.(HomeworkContent)
	quiz, isQuiz := l.content.(QuizContent)
	homework, isHomework := l.content.(HomeworkContent)
	if wasQuiz && !isQuiz {
		if err := deleteQuiz(ctx, ex, l.id); err != nil {
			return err
		}
	}
	if wasHomework && !isHomework {
		if err := deleteHomework(ctx, ex, l.id); err != nil {
			return err
		}
	}
	switch {
	case isQuiz:
		return saveQuiz(ctx, ex, l.id, quiz, oldQuiz, wasQuiz)
	case isHomework:
		return saveHomework(ctx, ex, l.id, homework, oldHomework, wasHomework)
	}
	return nil
}

// deleteQuiz xóa cấu hình và câu hỏi (CASCADE phương án) khi học liệu thôi là quiz.
func deleteQuiz(ctx context.Context, ex db.Executor, lessonID uuid.UUID) error {
	err := db.ExecAffectOne(ctx, ex, `DELETE FROM quiz_settings t WHERE t.lesson_id = $1 AND `+draftLessonOfT, lessonID)
	if err := immutableIfNoRows(err, "xóa cấu hình quiz"); err != nil {
		return err
	}
	if _, err := ex.ExecContext(ctx, `DELETE FROM questions t WHERE t.lesson_id = $1 AND `+draftLessonOfT, lessonID); err != nil {
		return lessonWriteError(err, "xóa câu hỏi")
	}
	return nil
}

// deleteHomework xóa đề, tiêu chí và file kèm khi học liệu thôi là homework.
func deleteHomework(ctx context.Context, ex db.Executor, lessonID uuid.UUID) error {
	err := db.ExecAffectOne(ctx, ex, `DELETE FROM homework_settings t WHERE t.lesson_id = $1 AND `+draftLessonOfT, lessonID)
	if err := immutableIfNoRows(err, "xóa đề homework"); err != nil {
		return err
	}
	if _, err := ex.ExecContext(ctx, `DELETE FROM rubric_criteria t WHERE t.lesson_id = $1 AND `+draftLessonOfT, lessonID); err != nil {
		return lessonWriteError(err, "xóa tiêu chí chấm")
	}
	if _, err := ex.ExecContext(ctx, `DELETE FROM homework_attachments t WHERE t.lesson_id = $1 AND `+draftLessonOfT, lessonID); err != nil {
		return lessonWriteError(err, "xóa file kèm")
	}
	return nil
}

func saveQuiz(ctx context.Context, ex db.Executor, lessonID uuid.UUID, c, old QuizContent, existed bool) error {
	if !existed || !sameQuizSettings(c, old) {
		var seconds *int
		if c.TimeLimit != nil {
			s := int(*c.TimeLimit / time.Second)
			seconds = &s
		}
		err := db.ExecAffectOne(ctx, ex, `INSERT INTO quiz_settings (lesson_id, mode, max_attempts, time_limit_seconds, shuffle, pass_percent)
			SELECT l.id, $2::text, $3::int, $4::int, $5::boolean, $6::int`+fromDraftLesson+`
			ON CONFLICT (lesson_id) DO UPDATE SET mode = EXCLUDED.mode, max_attempts = EXCLUDED.max_attempts,
				time_limit_seconds = EXCLUDED.time_limit_seconds, shuffle = EXCLUDED.shuffle, pass_percent = EXCLUDED.pass_percent`,
			lessonID, string(c.Mode), c.MaxAttempts, seconds, c.Shuffle, c.PassPercent)
		if err := immutableIfNoRows(err, "lưu cấu hình quiz"); err != nil {
			return err
		}
	}
	return saveQuestions(ctx, ex, lessonID, c.Questions, old.Questions)
}

func sameQuizSettings(a, b QuizContent) bool {
	return a.Mode == b.Mode && a.Shuffle == b.Shuffle && eqPtr(a.MaxAttempts, b.MaxAttempts) &&
		eqPtr(a.TimeLimit, b.TimeLimit) && eqPtr(a.PassPercent, b.PassPercent)
}

// saveQuestions diff câu hỏi theo id: xóa câu bị bỏ trước (nhả question_key cho câu thêm lại), rồi thêm câu mới và
// sửa câu đổi nội dung hoặc vị trí, kèm diff phương án của từng câu.
func saveQuestions(ctx context.Context, ex db.Executor, lessonID uuid.UUID, questions, old []*Question) error {
	oldByID := make(map[uuid.UUID]int, len(old))
	for i, q := range old {
		oldByID[q.ID] = i
	}
	wanted := make(map[uuid.UUID]bool, len(questions))
	for _, q := range questions {
		wanted[q.ID] = true
	}
	for _, q := range old {
		if wanted[q.ID] {
			continue
		}
		err := db.ExecAffectOne(ctx, ex, `DELETE FROM questions t WHERE t.id = $1 AND t.lesson_id = $2 AND `+draftLessonOfT, q.ID, lessonID)
		if err := immutableIfNoRows(err, "xóa câu hỏi"); err != nil {
			return err
		}
	}
	for i, q := range questions {
		pos := i + 1
		oi, ok := oldByID[q.ID]
		if !ok {
			err := db.ExecAffectOne(ctx, ex, `INSERT INTO questions (id, lesson_id, question_key, type, body_source, body_html,
				explanation_source, explanation_html, points, position)
				SELECT $2::uuid, l.id, $3::text, $4::text, $5::text, $6::text, $7::text, $8::text, $9::int, $10::int`+fromDraftLesson,
				lessonID, q.ID, q.Key.String(), string(q.Type), q.BodySource, q.BodyHTML, q.ExplanationSource, q.ExplanationHTML,
				q.Points, pos)
			if err := immutableIfNoRows(err, "thêm câu hỏi"); err != nil {
				return err
			}
			if err := saveOptions(ctx, ex, q.ID, q.Options, nil); err != nil {
				return err
			}
			continue
		}
		prev := old[oi]
		if oi+1 != pos || prev.Type != q.Type || prev.BodySource != q.BodySource || !eqPtr(prev.BodyHTML, q.BodyHTML) ||
			prev.ExplanationSource != q.ExplanationSource || !eqPtr(prev.ExplanationHTML, q.ExplanationHTML) || prev.Points != q.Points {
			err := db.ExecAffectOne(ctx, ex, `UPDATE questions t SET type = $3, body_source = $4, body_html = $5,
				explanation_source = $6, explanation_html = $7, points = $8, position = $9
				WHERE t.id = $1 AND t.lesson_id = $2 AND `+draftLessonOfT,
				q.ID, lessonID, string(q.Type), q.BodySource, q.BodyHTML, q.ExplanationSource, q.ExplanationHTML, q.Points, pos)
			if err := immutableIfNoRows(err, "sửa câu hỏi"); err != nil {
				return err
			}
		}
		if err := saveOptions(ctx, ex, q.ID, q.Options, prev.Options); err != nil {
			return err
		}
	}
	return nil
}

// saveOptions diff phương án của một câu hỏi theo id.
func saveOptions(ctx context.Context, ex db.Executor, questionID uuid.UUID, options, old []Option) error {
	oldByID := make(map[uuid.UUID]int, len(old))
	for i, o := range old {
		oldByID[o.ID] = i
	}
	for _, o := range old {
		if slices.ContainsFunc(options, func(n Option) bool { return n.ID == o.ID }) {
			continue
		}
		err := db.ExecAffectOne(ctx, ex, `DELETE FROM question_options t WHERE t.id = $1 AND t.question_id = $2 AND `+draftQuestionOfT,
			o.ID, questionID)
		if err := immutableIfNoRows(err, "xóa phương án"); err != nil {
			return err
		}
	}
	for i, o := range options {
		pos := i + 1
		oi, ok := oldByID[o.ID]
		switch {
		case !ok:
			err := db.ExecAffectOne(ctx, ex, `INSERT INTO question_options (id, question_id, body, is_correct, position)
				SELECT $2::uuid, q.id, $3::text, $4::boolean, $5::int FROM questions q JOIN lessons l ON l.id = q.lesson_id
				JOIN stage_versions sv ON sv.id = l.stage_version_id WHERE q.id = $1 AND sv.status = 'draft'`,
				questionID, o.ID, o.Body, o.Correct, pos)
			if err := immutableIfNoRows(err, "thêm phương án"); err != nil {
				return err
			}
		case oi+1 != pos || old[oi] != o:
			err := db.ExecAffectOne(ctx, ex, `UPDATE question_options t SET body = $3, is_correct = $4, position = $5
				WHERE t.id = $1 AND t.question_id = $2 AND `+draftQuestionOfT, o.ID, questionID, o.Body, o.Correct, pos)
			if err := immutableIfNoRows(err, "sửa phương án"); err != nil {
				return err
			}
		}
	}
	return nil
}

func saveHomework(ctx context.Context, ex db.Executor, lessonID uuid.UUID, c, old HomeworkContent, existed bool) error {
	if !existed || c.BriefSource != old.BriefSource || !eqPtr(c.BriefHTML, old.BriefHTML) || c.PassPercent != old.PassPercent {
		err := db.ExecAffectOne(ctx, ex, `INSERT INTO homework_settings (lesson_id, brief_source, brief_html, pass_percent)
			SELECT l.id, $2::text, $3::text, $4::int`+fromDraftLesson+`
			ON CONFLICT (lesson_id) DO UPDATE SET brief_source = EXCLUDED.brief_source, brief_html = EXCLUDED.brief_html,
				pass_percent = EXCLUDED.pass_percent`,
			lessonID, c.BriefSource, c.BriefHTML, c.PassPercent)
		if err := immutableIfNoRows(err, "lưu đề homework"); err != nil {
			return err
		}
	}
	if err := saveCriteria(ctx, ex, lessonID, c.Criteria, old.Criteria); err != nil {
		return err
	}
	return saveAttachments(ctx, ex, lessonID, c.Attachments, old.Attachments)
}

// saveCriteria diff tiêu chí chấm theo id.
func saveCriteria(ctx context.Context, ex db.Executor, lessonID uuid.UUID, criteria, old []*Criterion) error {
	oldByID := make(map[uuid.UUID]int, len(old))
	for i, c := range old {
		oldByID[c.ID] = i
	}
	for _, c := range old {
		if slices.ContainsFunc(criteria, func(n *Criterion) bool { return n.ID == c.ID }) {
			continue
		}
		err := db.ExecAffectOne(ctx, ex, `DELETE FROM rubric_criteria t WHERE t.id = $1 AND t.lesson_id = $2 AND `+draftLessonOfT,
			c.ID, lessonID)
		if err := immutableIfNoRows(err, "xóa tiêu chí chấm"); err != nil {
			return err
		}
	}
	for i, c := range criteria {
		pos := i + 1
		oi, ok := oldByID[c.ID]
		switch {
		case !ok:
			err := db.ExecAffectOne(ctx, ex, `INSERT INTO rubric_criteria (id, lesson_id, description, max_points, position)
				SELECT $2::uuid, l.id, $3::text, $4::int, $5::int`+fromDraftLesson, lessonID, c.ID, c.Description, c.MaxPoints, pos)
			if err := immutableIfNoRows(err, "thêm tiêu chí chấm"); err != nil {
				return err
			}
		case oi+1 != pos || *old[oi] != *c:
			err := db.ExecAffectOne(ctx, ex, `UPDATE rubric_criteria t SET description = $3, max_points = $4, position = $5
				WHERE t.id = $1 AND t.lesson_id = $2 AND `+draftLessonOfT, c.ID, lessonID, c.Description, c.MaxPoints, pos)
			if err := immutableIfNoRows(err, "sửa tiêu chí chấm"); err != nil {
				return err
			}
		}
	}
	return nil
}

// saveAttachments ghi lại toàn bộ file kèm khi danh sách hoặc thứ tự đổi: UNIQUE vị trí của homework_attachments
// không hoãn được nên xóa hết rồi chèn lại theo thứ tự mới.
func saveAttachments(ctx context.Context, ex db.Executor, lessonID uuid.UUID, attachments, old []Attachment) error {
	ids := func(as []Attachment) []uuid.UUID {
		out := make([]uuid.UUID, len(as))
		for i, a := range as {
			out[i] = a.MediaID
		}
		return out
	}
	if slices.Equal(ids(attachments), ids(old)) {
		return nil
	}
	if _, err := ex.ExecContext(ctx, `DELETE FROM homework_attachments t WHERE t.lesson_id = $1 AND `+draftLessonOfT, lessonID); err != nil {
		return lessonWriteError(err, "xóa file kèm")
	}
	for i, a := range attachments {
		err := db.ExecAffectOne(ctx, ex, `INSERT INTO homework_attachments (lesson_id, media_id, position)
			SELECT l.id, $2::uuid, $3::int`+fromDraftLesson, lessonID, a.MediaID, i+1)
		if err := immutableIfNoRows(err, "thêm file kèm"); err != nil {
			return err
		}
	}
	return nil
}

// contentPositionConstraints là các UNIQUE vị trí của bảng con được hoãn tới commit khi SaveDraft đổi thứ tự.
const contentPositionConstraints = pgerr.UqQuestionsLessonPosition + `, ` + pgerr.UqQuestionOptionsQuestionPosition + `, ` +
	pgerr.UqRubricCriteriaLessonPosition
