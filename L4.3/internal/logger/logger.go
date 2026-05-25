package logger

import (
	"context"
	"log/slog"
	"time"
)

// Entry описывает одну запись для асинхронного логгера.
type Entry struct {
	Message string
	Attrs   []slog.Attr
}

// AsyncLogger принимает логи в канал и пишет их из отдельной горутины.
type AsyncLogger struct {
	ch chan Entry
}

// New создаёт логгер с буфером заданного размера.
func New(buffer int) *AsyncLogger {
	if buffer <= 0 {
		buffer = 100
	}
	return &AsyncLogger{ch: make(chan Entry, buffer)}
}

// Channel возвращает канал для прямой отправки записей.
func (l *AsyncLogger) Channel() chan<- Entry {
	return l.ch
}

// Info кладёт информационную запись в очередь логов.
func (l *AsyncLogger) Info(message string, attrs ...slog.Attr) {
	entry := Entry{Message: message, Attrs: attrs}
	select {
	case l.ch <- entry:
	default:
		// Если буфер заполнен, не блокируем HTTP-хендлер.
		go func() {
			l.ch <- entry
		}()
	}
}

// Run обрабатывает очередь логов до отмены контекста.
func (l *AsyncLogger) Run(ctx context.Context, slogger *slog.Logger) {
	if slogger == nil {
		slogger = slog.Default()
	}

	for {
		select {
		case <-ctx.Done():
			l.drain(slogger)
			return
		case entry := <-l.ch:
			slogger.LogAttrs(ctx, slog.LevelInfo, entry.Message, entry.Attrs...)
		}
	}
}

// drain дописывает оставшиеся записи перед остановкой.
func (l *AsyncLogger) drain(slogger *slog.Logger) {
	timeout := time.After(500 * time.Millisecond)
	for {
		select {
		case entry := <-l.ch:
			slogger.LogAttrs(context.Background(), slog.LevelInfo, entry.Message, entry.Attrs...)
		case <-timeout:
			return
		default:
			return
		}
	}
}
