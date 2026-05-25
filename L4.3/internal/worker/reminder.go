package worker

import (
	"context"
	"log/slog"
	"time"

	"L4.3/internal/domain"
	"L4.3/internal/logger"
)

// ReminderMarker описывает сервис, который отмечает отправленные напоминания.
type ReminderMarker interface {
	MarkReminded(ctx context.Context, id int64, remindedAt time.Time) error
}

// RunReminderWorker читает задачи из канала и запускает отправку напоминаний.
func RunReminderWorker(
	ctx context.Context,
	service ReminderMarker,
	tasks <-chan domain.ReminderTask,
	log *logger.AsyncLogger,
) {
	for {
		select {
		case <-ctx.Done():
			return
		case task := <-tasks:
			go handleReminder(ctx, service, task, log)
		}
	}
}

// handleReminder ждёт времени напоминания и логирует его отправку.
func handleReminder(ctx context.Context, service ReminderMarker, task domain.ReminderTask, log *logger.AsyncLogger) {
	wait := time.Until(task.RemindAt)
	if wait > 0 {
		timer := time.NewTimer(wait)
		defer timer.Stop()

		select {
		case <-ctx.Done():
			return
		case <-timer.C:
		}
	}

	now := time.Now()
	if log != nil {
		log.Info(
			"event reminder",
			slog.Int64("event_id", task.EventID),
			slog.Int64("user_id", task.UserID),
			slog.String("name", task.Name),
			slog.Time("remind_at", task.RemindAt),
		)
	}

	if err := service.MarkReminded(ctx, task.EventID, now); err != nil && log != nil {
		log.Info("mark reminded failed", slog.Int64("event_id", task.EventID), slog.String("err", err.Error()))
	}
}
