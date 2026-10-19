package stages

import (
	"fmt"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"

	"lms/api/internal/domain"
)

// maxUnitTitle khớp CHECK char_length(btrim(title)) BETWEEN 1 AND 120 của stage_units.
const maxUnitTitle = 120

// Lỗi nghiệp vụ của bài học; message lấy theo prototype/app.js.
var (
	ErrUnitTitleRequired  = domain.ErrInvalid.WithMsg("Nhập tên bài học.")
	ErrUnitTitleTooLong   = domain.ErrInvalid.WithMsg(fmt.Sprintf("Tên bài học tối đa %d ký tự.", maxUnitTitle))
	ErrUnitTitleTaken     = domain.ErrConflict.WithMsg("Tên bài học đã có trong phiên bản này.")
	ErrUnitNotFound       = domain.ErrNotFound.WithMsg("Không tìm thấy bài học.")
	ErrLessonUnitRequired = domain.ErrInvalid.WithMsg("Chọn bài học cho học liệu.")
	ErrNoUnits            = domain.ErrInvalid.WithMsg("Chặng cần ít nhất một bài học trước khi phát hành.")
)

// ErrEmptyUnit: bài học chưa có học liệu nào thì không phát hành được.
func ErrEmptyUnit(title string) error {
	return domain.ErrInvalid.WithMsg(fmt.Sprintf("Bài học \"%s\" chưa có học liệu.", title))
}

// Unit là bài học: nhóm học liệu trong một phiên bản chặng. position bắt đầu từ 1 và liên tục; unit_key giữ nguyên
// khi nhân bản để đối chiếu bài học giữa các phiên bản.
type Unit struct {
	id       uuid.UUID
	key      domain.LessonKey
	title    string
	position int
}

func (u *Unit) ID() uuid.UUID         { return u.id }
func (u *Unit) Key() domain.LessonKey { return u.key }
func (u *Unit) Title() string         { return u.title }
func (u *Unit) Position() int         { return u.position }

// UnitLessons là một bài học trong bố cục sắp xếp: học liệu của bài học theo thứ tự mới.
type UnitLessons struct {
	UnitID    uuid.UUID
	LessonIDs []uuid.UUID
}

// Units trả bản sao danh sách bài học đã sắp theo position.
func (v *StageVersion) Units() []*Unit { return append([]*Unit(nil), v.units...) }

// Unit tìm bài học theo id.
func (v *StageVersion) Unit(id uuid.UUID) (*Unit, error) {
	for _, u := range v.units {
		if u.id == id {
			return u, nil
		}
	}
	return nil, ErrUnitNotFound
}

// LessonsOf trả học liệu của một bài học theo thứ tự.
func (v *StageVersion) LessonsOf(unitID uuid.UUID) []*Lesson {
	var out []*Lesson
	for _, l := range v.lessons {
		if l.unitID == unitID {
			out = append(out, l)
		}
	}
	return out
}

// AddUnit thêm bài học vào cuối bản nháp; unit_key sinh từ tên như lesson_key.
func (v *StageVersion) AddUnit(id uuid.UUID, title string) (*Unit, error) {
	if err := v.ensureMutable(); err != nil {
		return nil, err
	}
	title, err := v.validUnitTitle(uuid.Nil, title)
	if err != nil {
		return nil, err
	}
	u := &Unit{id: id, key: uniqueKey(title, "bai-hoc", v.hasUnitKey), title: title, position: len(v.units) + 1}
	v.units = append(v.units, u)
	return u, nil
}

// RenameUnit đổi tên bài học; unit_key giữ nguyên.
func (v *StageVersion) RenameUnit(id uuid.UUID, title string) error {
	if err := v.ensureMutable(); err != nil {
		return err
	}
	u, err := v.Unit(id)
	if err != nil {
		return err
	}
	if u.title, err = v.validUnitTitle(id, title); err != nil {
		return err
	}
	return nil
}

// RemoveUnit xóa bài học cùng toàn bộ học liệu trong đó; trả số học liệu bị xóa để UI báo cho người soạn.
func (v *StageVersion) RemoveUnit(id uuid.UUID) (int, error) {
	if err := v.ensureMutable(); err != nil {
		return 0, err
	}
	if _, err := v.Unit(id); err != nil {
		return 0, err
	}
	v.units = slices.DeleteFunc(v.units, func(u *Unit) bool { return u.id == id })
	before := len(v.lessons)
	v.lessons = slices.DeleteFunc(v.lessons, func(l *Lesson) bool { return l.unitID == id })
	v.renumber()
	return before - len(v.lessons), nil
}

// ReorderUnits sắp lại bài học theo idsInOrder; danh sách phải là hoán vị đầy đủ của bài học hiện có. Học liệu đi
// theo bài học của nó.
func (v *StageVersion) ReorderUnits(idsInOrder []uuid.UUID) error {
	if err := v.ensureMutable(); err != nil {
		return err
	}
	if len(idsInOrder) != len(v.units) {
		return ErrReorderMismatch
	}
	byID := make(map[uuid.UUID]*Unit, len(v.units))
	for _, u := range v.units {
		byID[u.id] = u
	}
	ordered := make([]*Unit, 0, len(idsInOrder))
	for _, id := range idsInOrder {
		u, ok := byID[id]
		if !ok {
			return ErrReorderMismatch
		}
		delete(byID, id)
		ordered = append(ordered, u)
	}
	v.units = ordered
	v.renumber()
	return nil
}

// MoveLesson đưa học liệu xuống cuối bài học khác; cùng bài học thì giữ nguyên vị trí.
func (v *StageVersion) MoveLesson(lessonID, unitID uuid.UUID) error {
	if err := v.ensureMutable(); err != nil {
		return err
	}
	l, err := v.Lesson(lessonID)
	if err != nil {
		return err
	}
	if _, err := v.Unit(unitID); err != nil {
		return ErrLessonUnitRequired
	}
	if l.unitID == unitID {
		return nil
	}
	v.lessons = append(slices.DeleteFunc(v.lessons, func(x *Lesson) bool { return x.id == lessonID }), l)
	l.unitID = unitID
	v.renumber()
	return nil
}

// ReorderLessons áp bố cục đầy đủ: mỗi bài học và mỗi học liệu xuất hiện đúng một lần. Thứ tự bài học không đổi
// (đổi bằng ReorderUnits); học liệu có thể chuyển sang bài học khác.
func (v *StageVersion) ReorderLessons(layout []UnitLessons) error {
	if err := v.ensureMutable(); err != nil {
		return err
	}
	if len(layout) != len(v.units) {
		return ErrReorderMismatch
	}
	units := make(map[uuid.UUID]bool, len(v.units))
	for _, u := range v.units {
		units[u.id] = true
	}
	byID := make(map[uuid.UUID]*Lesson, len(v.lessons))
	for _, l := range v.lessons {
		byID[l.id] = l
	}
	type placement struct {
		lesson *Lesson
		unitID uuid.UUID
	}
	placed := make([]placement, 0, len(v.lessons))
	for _, ul := range layout {
		if !units[ul.UnitID] {
			return ErrReorderMismatch
		}
		delete(units, ul.UnitID)
		for _, id := range ul.LessonIDs {
			l, ok := byID[id]
			if !ok {
				return ErrReorderMismatch
			}
			delete(byID, id)
			placed = append(placed, placement{lesson: l, unitID: ul.UnitID})
		}
	}
	if len(byID) > 0 {
		return ErrReorderMismatch
	}
	v.lessons = v.lessons[:0:0]
	for _, p := range placed {
		p.lesson.unitID = p.unitID
		v.lessons = append(v.lessons, p.lesson)
	}
	v.renumber()
	return nil
}

// validUnitTitle trim và kiểm độ dài, rồi kiểm trùng tên với bài học khác (bỏ qua self) không phân biệt hoa thường
// như unique index lower(btrim(title)).
func (v *StageVersion) validUnitTitle(self uuid.UUID, title string) (string, error) {
	title = strings.TrimSpace(title)
	switch {
	case title == "":
		return "", ErrUnitTitleRequired
	case utf8.RuneCountInString(title) > maxUnitTitle:
		return "", ErrUnitTitleTooLong
	}
	folded := strings.ToLower(title)
	for _, u := range v.units {
		if u.id != self && strings.ToLower(u.title) == folded {
			return "", ErrUnitTitleTaken
		}
	}
	return title, nil
}

func (v *StageVersion) hasUnitKey(key domain.LessonKey) bool {
	return slices.ContainsFunc(v.units, func(u *Unit) bool { return u.key == key })
}
