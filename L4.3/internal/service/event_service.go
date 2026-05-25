package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"L4.3/internal/domain"
	"L4.3/internal/repo"
)

const (
	dateLayout     = "2006-01-02"
	timeLayout     = "15:04"
	reminderLayout = time.RFC3339
)

// EventService содержит бизнес-логику календаря.
type EventService struct {
	repo      repo.EventRepo
	reminders chan<- domain.ReminderTask
	now       func() time.Time
}

// NewEventService создаёт сервис событий.
func NewEventService(rp repo.EventRepo, reminders chan<- domain.ReminderTask) *EventService {
	return &EventService{
		repo:      rp,
		reminders: reminders,
		now:       time.Now,
	}
}

// NewEventServiceWithClock создаёт сервис с подменяемыми часами для тестов.
func NewEventServiceWithClock(rp repo.EventRepo, reminders chan<- domain.ReminderTask, now func() time.Time) *EventService {
	srv := NewEventService(rp, reminders)
	srv.now = now
	return srv
}

// CreateEvent валидирует и создаёт событие.
func (s *EventService) CreateEvent(ctx context.Context, event domain.Event) (domain.Event, error) {
	if err := s.validateEvent(event); err != nil {
		return domain.Event{}, err
	}

	created, err := s.repo.Create(ctx, event)
	if err != nil {
		return domain.Event{}, err
	}
	s.enqueueReminder(created)

	return created, nil
}

// UpdateEvent обновляет активное событие.
func (s *EventService) UpdateEvent(ctx context.Context, id int64, event domain.Event) (domain.Event, error) {
	if id <= 0 {
		return domain.Event{}, fmt.Errorf("%w: id must be positive", ErrInvalidInput)
	}
	if err := s.validateEvent(event); err != nil {
		return domain.Event{}, err
	}

	updated, err := s.repo.Update(ctx, id, event)
	if err != nil {
		return domain.Event{}, mapRepoErr(err)
	}
	s.enqueueReminder(updated)

	return updated, nil
}

// DeleteEvent удаляет событие по ID.
func (s *EventService) DeleteEvent(ctx context.Context, id int64) (domain.Event, error) {
	if id <= 0 {
		return domain.Event{}, fmt.Errorf("%w: id must be positive", ErrInvalidInput)
	}

	deleted, err := s.repo.Delete(ctx, id)
	if err != nil {
		return domain.Event{}, mapRepoErr(err)
	}

	return deleted, nil
}

// GetEventsForDay возвращает события за календарный день.
func (s *EventService) GetEventsForDay(ctx context.Context, userID int64, dayStr string) ([]domain.Event, error) {
	day, err := parseDate(dayStr)
	if err != nil {
		return nil, err
	}
	return s.list(ctx, userID, day, day.AddDate(0, 0, 1))
}

// GetEventsForWeek возвращает события за ISO-неделю.
func (s *EventService) GetEventsForWeek(ctx context.Context, userID int64, dayStr string) ([]domain.Event, error) {
	day, err := parseDate(dayStr)
	if err != nil {
		return nil, err
	}

	weekday := int(day.Weekday())
	if weekday == 0 {
		weekday = 7
	}
	start := day.AddDate(0, 0, -(weekday - 1))
	return s.list(ctx, userID, start, start.AddDate(0, 0, 7))
}

// GetEventsForMonth возвращает события за месяц.
func (s *EventService) GetEventsForMonth(ctx context.Context, userID int64, dayStr string) ([]domain.Event, error) {
	day, err := parseDate(dayStr)
	if err != nil {
		return nil, err
	}
	start := time.Date(day.Year(), day.Month(), 1, 0, 0, 0, 0, day.Location())
	return s.list(ctx, userID, start, start.AddDate(0, 1, 0))
}

// ArchiveOldEvents переносит старые события в архив.
func (s *EventService) ArchiveOldEvents(ctx context.Context, before time.Time) (int64, error) {
	return s.repo.ArchiveOld(ctx, before)
}

// PendingReminders возвращает напоминания, которые надо восстановить при старте.
func (s *EventService) PendingReminders(ctx context.Context, now time.Time) ([]domain.Event, error) {
	return s.repo.ListPendingReminders(ctx, now)
}

// MarkReminded отмечает отправленное напоминание.
func (s *EventService) MarkReminded(ctx context.Context, id int64, remindedAt time.Time) error {
	return mapRepoErr(s.repo.MarkReminded(ctx, id, remindedAt))
}

// EnqueueExistingReminder кладёт восстановленное напоминание в канал воркера.
func (s *EventService) EnqueueExistingReminder(event domain.Event) {
	s.enqueueReminder(event)
}

func (s *EventService) list(ctx context.Context, userID int64, from time.Time, to time.Time) ([]domain.Event, error) {
	if userID <= 0 {
		return nil, fmt.Errorf("%w: user_id must be positive", ErrInvalidInput)
	}

	events, err := s.repo.ListForPeriod(ctx, userID, from, to)
	if err != nil {
		return nil, err
	}

	active := make([]domain.Event, 0, len(events))
	for _, event := range events {
		if !event.IsArchived() {
			active = append(active, event)
		}
	}

	return active, nil
}

// validateEvent проверяет обязательные поля и форматы дат.
func (s *EventService) validateEvent(event domain.Event) error {
	if event.UserID <= 0 {
		return fmt.Errorf("%w: user_id must be positive", ErrInvalidInput)
	}
	if strings.TrimSpace(event.Name) == "" {
		return fmt.Errorf("%w: name is required", ErrInvalidInput)
	}
	if _, err := parseDate(event.Date); err != nil {
		return err
	}
	if _, err := time.Parse(timeLayout, event.Time); err != nil {
		return fmt.Errorf("%w: time must use HH:MM format", ErrInvalidInput)
	}
	if event.RemindAt != nil && !event.RemindAt.After(s.now()) {
		return fmt.Errorf("%w: remind_at must be in future", ErrInvalidInput)
	}
	return nil
}

// enqueueReminder ставит задачу напоминания в канал.
func (s *EventService) enqueueReminder(event domain.Event) {
	if event.RemindAt == nil || s.reminders == nil {
		return
	}

	task := domain.ReminderTask{
		EventID:  event.ID,
		UserID:   event.UserID,
		Name:     event.Name,
		RemindAt: *event.RemindAt,
	}

	select {
	case s.reminders <- task:
	default:
		go func() {
			s.reminders <- task
		}()
	}
}

func parseDate(value string) (time.Time, error) {
	day, err := time.Parse(dateLayout, value)
	if err != nil {
		return time.Time{}, fmt.Errorf("%w: date must use YYYY-MM-DD format", ErrInvalidInput)
	}
	return day, nil
}

// mapRepoErr скрывает ошибки хранилища за ошибками сервисного слоя.
func mapRepoErr(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, repo.ErrNotFound) {
		return ErrNotFound
	}
	return err
}
