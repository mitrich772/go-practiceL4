package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"L4.3/internal/domain"
	"L4.3/internal/repo"
	"L4.3/internal/repo/mocks"
	"github.com/golang/mock/gomock"
)

func TestCreateEventWithoutReminder(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	rp := mocks.NewMockEventRepo(ctrl)
	reminders := make(chan domain.ReminderTask, 1)
	now := time.Date(2026, 5, 25, 12, 0, 0, 0, time.UTC)
	srv := NewEventServiceWithClock(rp, reminders, func() time.Time { return now })

	input := domain.Event{UserID: 1, Date: "2026-05-26", Time: "10:30", Name: "exam"}
	created := input
	created.ID = 10

	rp.EXPECT().Create(gomock.Any(), input).Return(created, nil)

	got, err := srv.CreateEvent(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.ID != 10 {
		t.Fatalf("expected id 10, got %d", got.ID)
	}
	if len(reminders) != 0 {
		t.Fatalf("expected no reminder tasks, got %d", len(reminders))
	}
}

func TestCreateEventWithReminderEnqueuesTask(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	rp := mocks.NewMockEventRepo(ctrl)
	reminders := make(chan domain.ReminderTask, 1)
	now := time.Date(2026, 5, 25, 12, 0, 0, 0, time.UTC)
	remindAt := now.Add(time.Hour)
	srv := NewEventServiceWithClock(rp, reminders, func() time.Time { return now })

	input := domain.Event{UserID: 1, Date: "2026-05-26", Time: "10:30", Name: "exam", RemindAt: &remindAt}
	created := input
	created.ID = 11

	rp.EXPECT().Create(gomock.Any(), input).Return(created, nil)

	if _, err := srv.CreateEvent(context.Background(), input); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	select {
	case task := <-reminders:
		if task.EventID != 11 || task.UserID != 1 || task.Name != "exam" || !task.RemindAt.Equal(remindAt) {
			t.Fatalf("unexpected task: %+v", task)
		}
	default:
		t.Fatal("expected reminder task")
	}
}

func TestCreateEventWithPastReminderReturnsInputError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	rp := mocks.NewMockEventRepo(ctrl)
	now := time.Date(2026, 5, 25, 12, 0, 0, 0, time.UTC)
	remindAt := now.Add(-time.Minute)
	srv := NewEventServiceWithClock(rp, nil, func() time.Time { return now })

	input := domain.Event{UserID: 1, Date: "2026-05-26", Time: "10:30", Name: "exam", RemindAt: &remindAt}
	_, err := srv.CreateEvent(context.Background(), input)
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
}

func TestDeleteNotFoundReturnsServiceError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	rp := mocks.NewMockEventRepo(ctrl)
	srv := NewEventService(rp, nil)

	rp.EXPECT().Delete(gomock.Any(), int64(42)).Return(domain.Event{}, repo.ErrNotFound)

	_, err := srv.DeleteEvent(context.Background(), 42)
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestUpdateNotFoundReturnsServiceError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	rp := mocks.NewMockEventRepo(ctrl)
	srv := NewEventService(rp, nil)
	input := domain.Event{UserID: 1, Date: "2026-05-26", Time: "10:30", Name: "exam"}

	rp.EXPECT().Update(gomock.Any(), int64(42), input).Return(domain.Event{}, repo.ErrNotFound)

	_, err := srv.UpdateEvent(context.Background(), 42, input)
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestGetEventsForDayUsesDayBounds(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	rp := mocks.NewMockEventRepo(ctrl)
	srv := NewEventService(rp, nil)

	from := time.Date(2026, 5, 25, 0, 0, 0, 0, time.UTC)
	to := from.AddDate(0, 0, 1)
	expected := []domain.Event{{ID: 1, UserID: 7, Date: "2026-05-25", Time: "10:00", Name: "active"}}

	rp.EXPECT().ListForPeriod(gomock.Any(), int64(7), from, to).Return(expected, nil)

	got, err := srv.GetEventsForDay(context.Background(), 7, "2026-05-25")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 || got[0].ID != 1 {
		t.Fatalf("unexpected events: %+v", got)
	}
}

func TestGetEventsForDaySkipsArchivedEvents(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	rp := mocks.NewMockEventRepo(ctrl)
	srv := NewEventService(rp, nil)
	from := time.Date(2026, 5, 25, 0, 0, 0, 0, time.UTC)
	to := from.AddDate(0, 0, 1)
	archivedAt := time.Date(2026, 5, 25, 13, 0, 0, 0, time.UTC)

	rp.EXPECT().ListForPeriod(gomock.Any(), int64(7), from, to).Return([]domain.Event{
		{ID: 1, UserID: 7, Date: "2026-05-25", Time: "10:00", Name: "active"},
		{ID: 2, UserID: 7, Date: "2026-05-25", Time: "11:00", Name: "archived", ArchivedAt: &archivedAt},
	}, nil)

	got, err := srv.GetEventsForDay(context.Background(), 7, "2026-05-25")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 || got[0].ID != 1 {
		t.Fatalf("unexpected events: %+v", got)
	}
}

func TestArchiveOldEventsDelegatesToRepo(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	rp := mocks.NewMockEventRepo(ctrl)
	srv := NewEventService(rp, nil)
	before := time.Date(2026, 5, 25, 12, 0, 0, 0, time.UTC)

	rp.EXPECT().ArchiveOld(gomock.Any(), before).Return(int64(3), nil)

	got, err := srv.ArchiveOldEvents(context.Background(), before)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != 3 {
		t.Fatalf("expected 3 archived events, got %d", got)
	}
}
