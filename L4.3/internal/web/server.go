package web

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"L4.3/internal/domain"
	"L4.3/internal/logger"
	"L4.3/internal/service"
)

// EventService описывает методы бизнес-логики, нужные HTTP-слою.
type EventService interface {
	CreateEvent(ctx context.Context, event domain.Event) (domain.Event, error)
	UpdateEvent(ctx context.Context, id int64, event domain.Event) (domain.Event, error)
	DeleteEvent(ctx context.Context, id int64) (domain.Event, error)
	GetEventsForDay(ctx context.Context, userID int64, dayStr string) ([]domain.Event, error)
	GetEventsForWeek(ctx context.Context, userID int64, dayStr string) ([]domain.Event, error)
	GetEventsForMonth(ctx context.Context, userID int64, dayStr string) ([]domain.Event, error)
}

// EventServer содержит HTTP-хендлеры календаря.
type EventServer struct {
	service EventService
	log     *logger.AsyncLogger
}

// New создаёт HTTP-сервер событий.
func New(service EventService, log *logger.AsyncLogger) *EventServer {
	return &EventServer{service: service, log: log}
}

// Routes регистрирует маршруты сервиса.
func (s *EventServer) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/create_event", s.withLogging(s.CreateEvent))
	mux.HandleFunc("/update_event", s.withLogging(s.UpdateEvent))
	mux.HandleFunc("/delete_event", s.withLogging(s.DeleteEvent))
	mux.HandleFunc("/events_for_day", s.withLogging(s.GetEventsForDay))
	mux.HandleFunc("/events_for_week", s.withLogging(s.GetEventsForWeek))
	mux.HandleFunc("/events_for_month", s.withLogging(s.GetEventsForMonth))
	return mux
}

// CreateEvent обрабатывает создание события.
func (s *EventServer) CreateEvent(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusBadRequest, "method must be POST")
		return
	}

	var input domain.Event
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	created, err := s.service.CreateEvent(r.Context(), input)
	if err != nil {
		s.writeServiceError(w, err)
		return
	}

	writeResult(w, http.StatusOK, created)
}

// UpdateEvent обрабатывает обновление события.
func (s *EventServer) UpdateEvent(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusBadRequest, "method must be POST")
		return
	}

	id, ok := parseID(w, r)
	if !ok {
		return
	}

	var input domain.Event
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	updated, err := s.service.UpdateEvent(r.Context(), id, input)
	if err != nil {
		s.writeServiceError(w, err)
		return
	}

	writeResult(w, http.StatusOK, updated)
}

// DeleteEvent обрабатывает удаление события.
func (s *EventServer) DeleteEvent(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusBadRequest, "method must be POST")
		return
	}

	id, ok := parseID(w, r)
	if !ok {
		return
	}

	deleted, err := s.service.DeleteEvent(r.Context(), id)
	if err != nil {
		s.writeServiceError(w, err)
		return
	}

	writeResult(w, http.StatusOK, deleted)
}

// GetEventsForDay возвращает события за день.
func (s *EventServer) GetEventsForDay(w http.ResponseWriter, r *http.Request) {
	s.getEvents(w, r, s.service.GetEventsForDay)
}

// GetEventsForWeek возвращает события за неделю.
func (s *EventServer) GetEventsForWeek(w http.ResponseWriter, r *http.Request) {
	s.getEvents(w, r, s.service.GetEventsForWeek)
}

// GetEventsForMonth возвращает события за месяц.
func (s *EventServer) GetEventsForMonth(w http.ResponseWriter, r *http.Request) {
	s.getEvents(w, r, s.service.GetEventsForMonth)
}

func (s *EventServer) getEvents(
	w http.ResponseWriter,
	r *http.Request,
	list func(context.Context, int64, string) ([]domain.Event, error),
) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusBadRequest, "method must be GET")
		return
	}

	userIDStr := r.URL.Query().Get("user_id")
	dateStr := r.URL.Query().Get("date")
	if userIDStr == "" || dateStr == "" {
		writeError(w, http.StatusBadRequest, "user_id and date are required")
		return
	}

	userID, err := strconv.ParseInt(userIDStr, 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid user_id")
		return
	}

	events, err := list(r.Context(), userID, dateStr)
	if err != nil {
		s.writeServiceError(w, err)
		return
	}

	writeResult(w, http.StatusOK, events)
}

func (s *EventServer) withLogging(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next(w, r)
		if s.log != nil {
			s.log.Info(
				"http request",
				slog.String("method", r.Method),
				slog.String("url", r.URL.String()),
				slog.Duration("duration", time.Since(start)),
			)
		}
	}
}

func (s *EventServer) writeServiceError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, service.ErrInvalidInput):
		writeError(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, service.ErrNotFound):
		writeError(w, http.StatusServiceUnavailable, err.Error())
	default:
		if s.log != nil {
			s.log.Info("handler error", slog.String("err", err.Error()))
		}
		writeError(w, http.StatusInternalServerError, "internal error")
	}
}

func parseID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	idStr := r.URL.Query().Get("id")
	if idStr == "" {
		writeError(w, http.StatusBadRequest, "id is required")
		return 0, false
	}

	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return 0, false
	}

	return id, true
}

func writeError(w http.ResponseWriter, code int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(ErrorResponse{Error: msg})
}

func writeResult(w http.ResponseWriter, code int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(ResultResponse{Result: data})
}
