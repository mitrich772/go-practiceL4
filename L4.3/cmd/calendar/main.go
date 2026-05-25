package main

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"L4.3/internal/config"
	"L4.3/internal/domain"
	asyncLog "L4.3/internal/logger"
	"L4.3/internal/repo/postgres"
	"L4.3/internal/service"
	"L4.3/internal/web"
	"L4.3/internal/worker"
	_ "github.com/lib/pq"
)

const (
	localEnv = "local"
	prodEnv  = "prod"
	devEnv   = "dev"
)

func main() {
	cfg := config.MustLoad("config/local.yaml")
	log := setupLogger(cfg.Env)

	db, err := openPostgres(cfg.Storage)
	if err != nil {
		log.Error("failed to connect postgres", slog.Any("err", err))
		os.Exit(1)
	}
	defer db.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	appLog := asyncLog.New(cfg.Workers.LogBuffer)
	go appLog.Run(ctx, log)

	// Канал связывает HTTP-создание событий и воркер напоминаний.
	reminderCh := make(chan domain.ReminderTask, cfg.Workers.ReminderBuffer)
	repo := postgres.New(db)
	eventService := service.NewEventService(repo, reminderCh)

	restorePendingReminders(ctx, eventService, appLog)

	go worker.RunReminderWorker(ctx, eventService, reminderCh, appLog)
	go worker.RunArchiveWorker(ctx, eventService, cfg.Workers.ArchiveEvery, appLog)

	handler := web.New(eventService, appLog)
	server := &http.Server{
		Addr:         cfg.HTTPServer.Address,
		Handler:      handler.Routes(),
		ReadTimeout:  cfg.HTTPServer.Timeout,
		WriteTimeout: cfg.HTTPServer.Timeout,
		IdleTimeout:  cfg.HTTPServer.IdleTimeout,
	}

	go func() {
		appLog.Info("server started", slog.String("address", cfg.HTTPServer.Address))
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error("server failed", slog.Any("err", err))
			cancel()
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop
	cancel()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Error("server shutdown failed", slog.Any("err", err))
	}
}

// openPostgres открывает соединение с PostgreSQL и проверяет доступность БД.
func openPostgres(cfg config.Storage) (*sql.DB, error) {
	dsn := fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		cfg.Host,
		cfg.Port,
		cfg.User,
		cfg.Password,
		cfg.DBName,
		cfg.SSLMode,
	)

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, err
	}

	db.SetMaxOpenConns(cfg.MaxOpenConns)
	db.SetMaxIdleConns(cfg.MaxIdleConns)
	db.SetConnMaxLifetime(cfg.ConnMaxLifetime)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}

	return db, nil
}

// restorePendingReminders восстанавливает будущие напоминания после перезапуска.
func restorePendingReminders(ctx context.Context, srv *service.EventService, log *asyncLog.AsyncLogger) {
	events, err := srv.PendingReminders(ctx, time.Now())
	if err != nil {
		log.Info("restore reminders failed", slog.String("err", err.Error()))
		return
	}
	for _, event := range events {
		srv.EnqueueExistingReminder(event)
	}
	log.Info("pending reminders restored", slog.Int("count", len(events)))
}

// setupLogger выбирает формат логов для окружения.
func setupLogger(env string) *slog.Logger {
	switch env {
	case prodEnv, devEnv:
		return slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	case localEnv:
		return slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	default:
		return slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	}
}
