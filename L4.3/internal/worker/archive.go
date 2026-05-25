package worker

import (
	"context"
	"log/slog"
	"time"

	"L4.3/internal/logger"
)

// Archiver описывает сервис, который умеет архивировать старые события.
type Archiver interface {
	ArchiveOldEvents(ctx context.Context, before time.Time) (int64, error)
}

// RunArchiveWorker периодически переносит старые события в архив.
func RunArchiveWorker(ctx context.Context, service Archiver, every time.Duration, log *logger.AsyncLogger) {
	if every <= 0 {
		every = time.Minute
	}

	ticker := time.NewTicker(every)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case now := <-ticker.C:
			archived, err := service.ArchiveOldEvents(ctx, now)
			if log == nil {
				continue
			}
			if err != nil {
				log.Info("archive failed", slog.String("err", err.Error()))
				continue
			}
			if archived > 0 {
				log.Info("events archived", slog.Int64("count", archived))
			}
		}
	}
}
