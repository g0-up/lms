package grading

import (
	"context"

	"lms/api/internal/platform/clock"
	"lms/api/internal/platform/db"
)

// NextFinder trả mục kế tiếp trong hàng chờ của actor sau khi xử lý xong mục after. homework gọi sau lần chấm đầu,
// quizzes sau lần xác nhận đầu; ex là transaction của thao tác đó để mục vừa xử lý đã rời hàng chờ và lỗi đọc làm
// rollback cả thao tác thay vì trả lỗi sau khi đã commit.
type NextFinder interface {
	Next(ctx context.Context, ex db.Executor, actor Actor, after QueueKey) (*QueueItem, error)
}

// Service là use case đọc hàng chờ chấm.
type Service struct {
	tx    db.Tx
	repo  Repo
	clock clock.Clock
}

var _ NextFinder = (*Service)(nil)

// Deps là phụ thuộc của Service. Tx nên là transaction chỉ đọc REPEATABLE READ để hàng chờ, số đếm và ô lọc thấy
// cùng một snapshot.
type Deps struct {
	Tx    db.Tx
	Repo  Repo
	Clock clock.Clock
}

// NewService nối phụ thuộc.
func NewService(d Deps) *Service {
	return &Service{tx: d.Tx, repo: d.Repo, clock: d.Clock}
}

// Queue là hàng chờ của actor đã lọc theo f, kèm số đếm trên toàn hàng chờ và các option lọc.
func (s *Service) Queue(ctx context.Context, actor Actor, f Filter) (Queue, error) {
	teacherID, err := actor.scope()
	if err != nil {
		return Queue{}, err
	}
	var out Queue
	err = s.tx.Transact(ctx, func(tx db.Executor) error {
		all, err := s.queue(ctx, tx, actor)
		if err != nil {
			return err
		}
		classes, err := s.repo.FilterClasses(ctx, tx, teacherID)
		if err != nil {
			return err
		}
		lessons, err := s.repo.FilterLessons(ctx, tx, teacherID)
		if err != nil {
			return err
		}
		out = buildQueue(all, f, classes, lessons)
		return nil
	})
	if err != nil {
		return Queue{}, err
	}
	return out, nil
}

// Recent là tối đa limit (1..MaxRecentLimit) việc đã chấm hoặc xác nhận gần đây ở các lớp actor thấy.
func (s *Service) Recent(ctx context.Context, actor Actor, limit int) ([]Recent, error) {
	teacherID, err := actor.scope()
	if err != nil {
		return nil, err
	}
	if limit < 1 || limit > MaxRecentLimit {
		return nil, ErrInvalidFilter
	}
	var out []Recent
	err = s.tx.Transact(ctx, func(tx db.Executor) error {
		out, err = s.repo.Recent(ctx, tx, teacherID, limit)
		return err
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// Next trả mục cũ nhất trong toàn hàng chờ của actor (không theo bộ lọc), bỏ qua after; hết mục trả nil.
func (s *Service) Next(ctx context.Context, ex db.Executor, actor Actor, after QueueKey) (*QueueItem, error) {
	all, err := s.queue(ctx, ex, actor)
	if err != nil {
		return nil, err
	}
	return next(all, after), nil
}

// queue đọc toàn hàng chờ của actor và điền thời gian chờ theo đồng hồ hiện tại.
func (s *Service) queue(ctx context.Context, ex db.Executor, actor Actor) ([]QueueItem, error) {
	teacherID, err := actor.scope()
	if err != nil {
		return nil, err
	}
	all, err := s.repo.Queue(ctx, ex, teacherID)
	if err != nil {
		return nil, err
	}
	return stamp(all, s.clock.Now()), nil
}
