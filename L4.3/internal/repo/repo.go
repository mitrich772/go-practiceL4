package repo

import (
	"context"
	"errors"
	"time"

	"L4.3/internal/domain"
)

var ErrNotFound = errors.New("event not found")

// EventRepo описывает операции хранилища событий.
//
//go:generate mockgen -source=repo.go -destination=./mocks/mock_repo.go -package=mocks
type EventRepo interface {
	Create(ctx context.Context, event domain.Event) (domain.Event, error)
	Update(ctx context.Context, id int64, event domain.Event) (domain.Event, error)
	Delete(ctx context.Context, id int64) (domain.Event, error)
	ListForPeriod(ctx context.Context, userID int64, from time.Time, to time.Time) ([]domain.Event, error)
	ArchiveOld(ctx context.Context, before time.Time) (int64, error)
	ListPendingReminders(ctx context.Context, now time.Time) ([]domain.Event, error)
	MarkReminded(ctx context.Context, id int64, remindedAt time.Time) error
}
