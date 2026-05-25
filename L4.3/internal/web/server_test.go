package web

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"L4.3/internal/domain"
	"L4.3/internal/service"
)

type fakeService struct {
	create func(context.Context, domain.Event) (domain.Event, error)
	list   func(context.Context, int64, string) ([]domain.Event, error)
}

func (f fakeService) CreateEvent(ctx context.Context, event domain.Event) (domain.Event, error) {
	return f.create(ctx, event)
}

func (f fakeService) UpdateEvent(context.Context, int64, domain.Event) (domain.Event, error) {
	return domain.Event{}, nil
}

func (f fakeService) DeleteEvent(context.Context, int64) (domain.Event, error) {
	return domain.Event{}, nil
}

func (f fakeService) GetEventsForDay(ctx context.Context, userID int64, dayStr string) ([]domain.Event, error) {
	return f.list(ctx, userID, dayStr)
}

func (f fakeService) GetEventsForWeek(ctx context.Context, userID int64, dayStr string) ([]domain.Event, error) {
	return f.list(ctx, userID, dayStr)
}

func (f fakeService) GetEventsForMonth(ctx context.Context, userID int64, dayStr string) ([]domain.Event, error) {
	return f.list(ctx, userID, dayStr)
}

func TestCreateEventSuccess(t *testing.T) {
	srv := New(fakeService{
		create: func(_ context.Context, event domain.Event) (domain.Event, error) {
			event.ID = 1
			return event, nil
		},
	}, nil)

	body := []byte(`{"user_id":1,"date":"2026-05-25","time":"10:00","name":"lesson"}`)
	req := httptest.NewRequest(http.MethodPost, "/create_event", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	srv.Routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp ResultResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid response: %v", err)
	}
	if resp.Result == nil {
		t.Fatal("expected result")
	}
}

func TestCreateEventValidationError(t *testing.T) {
	srv := New(fakeService{
		create: func(context.Context, domain.Event) (domain.Event, error) {
			return domain.Event{}, service.ErrInvalidInput
		},
	}, nil)

	req := httptest.NewRequest(http.MethodPost, "/create_event", bytes.NewReader([]byte(`{"user_id":0}`)))
	rec := httptest.NewRecorder()

	srv.Routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", rec.Code)
	}
}

func TestGetEventsForDaySuccess(t *testing.T) {
	srv := New(fakeService{
		list: func(_ context.Context, userID int64, dayStr string) ([]domain.Event, error) {
			if userID != 1 || dayStr != "2026-05-25" {
				t.Fatalf("unexpected args: user_id=%d date=%s", userID, dayStr)
			}
			return []domain.Event{{ID: 1, UserID: 1, Date: dayStr, Time: "10:00", Name: "lesson"}}, nil
		},
	}, nil)

	req := httptest.NewRequest(http.MethodGet, "/events_for_day?user_id=1&date=2026-05-25", nil)
	rec := httptest.NewRecorder()

	srv.Routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestNotFoundMapsToServiceUnavailable(t *testing.T) {
	srv := New(fakeService{
		create: func(context.Context, domain.Event) (domain.Event, error) {
			return domain.Event{}, service.ErrNotFound
		},
	}, nil)

	req := httptest.NewRequest(http.MethodPost, "/create_event", bytes.NewReader([]byte(`{"user_id":1}`)))
	rec := httptest.NewRecorder()

	srv.Routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected status 503, got %d", rec.Code)
	}
}

func TestInternalErrorMapsTo500(t *testing.T) {
	srv := New(fakeService{
		create: func(context.Context, domain.Event) (domain.Event, error) {
			return domain.Event{}, errors.New("db failed")
		},
	}, nil)

	req := httptest.NewRequest(http.MethodPost, "/create_event", bytes.NewReader([]byte(`{"user_id":1}`)))
	rec := httptest.NewRecorder()

	srv.Routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected status 500, got %d", rec.Code)
	}
}
