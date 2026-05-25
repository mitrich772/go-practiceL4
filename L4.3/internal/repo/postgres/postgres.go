package postgres

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"L4.3/internal/domain"
	"L4.3/internal/repo"
)

// PostgresRepo реализует хранилище событий на PostgreSQL.
type PostgresRepo struct {
	db *sql.DB
}

// New создаёт PostgreSQL-репозиторий.
func New(db *sql.DB) *PostgresRepo {
	return &PostgresRepo{db: db}
}

// Create сохраняет новое событие и возвращает запись с ID.
func (r *PostgresRepo) Create(ctx context.Context, event domain.Event) (domain.Event, error) {
	const query = `
		INSERT INTO events (user_id, event_date, event_time, name, remind_at)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, user_id, event_date, event_time, name, remind_at, reminded_at, archived_at, created_at, updated_at`

	return r.scanEvent(r.db.QueryRowContext(
		ctx,
		query,
		event.UserID,
		event.Date,
		event.Time,
		event.Name,
		event.RemindAt,
	))
}

// Update обновляет активное событие по ID.
func (r *PostgresRepo) Update(ctx context.Context, id int64, event domain.Event) (domain.Event, error) {
	const query = `
		UPDATE events
		SET user_id = $2,
			event_date = $3,
			event_time = $4,
			name = $5,
			remind_at = $6,
			reminded_at = NULL,
			updated_at = now()
		WHERE id = $1 AND archived_at IS NULL
		RETURNING id, user_id, event_date, event_time, name, remind_at, reminded_at, archived_at, created_at, updated_at`

	ev, err := r.scanEvent(r.db.QueryRowContext(
		ctx,
		query,
		id,
		event.UserID,
		event.Date,
		event.Time,
		event.Name,
		event.RemindAt,
	))
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Event{}, repo.ErrNotFound
	}
	return ev, err
}

// Delete удаляет событие по ID.
func (r *PostgresRepo) Delete(ctx context.Context, id int64) (domain.Event, error) {
	const query = `
		DELETE FROM events
		WHERE id = $1
		RETURNING id, user_id, event_date, event_time, name, remind_at, reminded_at, archived_at, created_at, updated_at`

	ev, err := r.scanEvent(r.db.QueryRowContext(ctx, query, id))
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Event{}, repo.ErrNotFound
	}
	return ev, err
}

// ListForPeriod возвращает активные события пользователя за период.
func (r *PostgresRepo) ListForPeriod(ctx context.Context, userID int64, from time.Time, to time.Time) ([]domain.Event, error) {
	const query = `
		SELECT id, user_id, event_date, event_time, name, remind_at, reminded_at, archived_at, created_at, updated_at
		FROM events
		WHERE user_id = $1
			AND archived_at IS NULL
			AND event_date >= $2
			AND event_date < $3
		ORDER BY event_date, event_time, id`

	rows, err := r.db.QueryContext(ctx, query, userID, from.Format("2006-01-02"), to.Format("2006-01-02"))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanEvents(rows)
}

// ArchiveOld переносит прошедшие события в архив.
func (r *PostgresRepo) ArchiveOld(ctx context.Context, before time.Time) (int64, error) {
	const query = `
		UPDATE events
		SET archived_at = now(), updated_at = now()
		WHERE archived_at IS NULL
			AND (event_date + event_time) < $1`

	res, err := r.db.ExecContext(ctx, query, before)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// ListPendingReminders возвращает будущие ненаправленные напоминания.
func (r *PostgresRepo) ListPendingReminders(ctx context.Context, now time.Time) ([]domain.Event, error) {
	const query = `
		SELECT id, user_id, event_date, event_time, name, remind_at, reminded_at, archived_at, created_at, updated_at
		FROM events
		WHERE remind_at IS NOT NULL
			AND reminded_at IS NULL
			AND archived_at IS NULL
			AND remind_at >= $1
		ORDER BY remind_at, id`

	rows, err := r.db.QueryContext(ctx, query, now)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanEvents(rows)
}

// MarkReminded отмечает, что напоминание по событию отправлено.
func (r *PostgresRepo) MarkReminded(ctx context.Context, id int64, remindedAt time.Time) error {
	const query = `
		UPDATE events
		SET reminded_at = $2, updated_at = now()
		WHERE id = $1 AND reminded_at IS NULL`

	res, err := r.db.ExecContext(ctx, query, id, remindedAt)
	if err != nil {
		return err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return repo.ErrNotFound
	}
	return nil
}

// scanEvent преобразует SQL-строку в доменную модель.
func (r *PostgresRepo) scanEvent(row interface {
	Scan(dest ...any) error
}) (domain.Event, error) {
	var (
		ev         domain.Event
		eventDate  time.Time
		eventTime  time.Time
		remindAt   sql.NullTime
		remindedAt sql.NullTime
		archivedAt sql.NullTime
	)

	err := row.Scan(
		&ev.ID,
		&ev.UserID,
		&eventDate,
		&eventTime,
		&ev.Name,
		&remindAt,
		&remindedAt,
		&archivedAt,
		&ev.CreatedAt,
		&ev.UpdatedAt,
	)
	if err != nil {
		return domain.Event{}, err
	}

	ev.Date = eventDate.Format("2006-01-02")
	ev.Time = eventTime.Format("15:04")
	if remindAt.Valid {
		ev.RemindAt = &remindAt.Time
	}
	if remindedAt.Valid {
		ev.RemindedAt = &remindedAt.Time
	}
	if archivedAt.Valid {
		ev.ArchivedAt = &archivedAt.Time
	}

	return ev, nil
}

// scanEvents читает набор строк с событиями.
func scanEvents(rows *sql.Rows) ([]domain.Event, error) {
	var events []domain.Event
	for rows.Next() {
		ev, err := (&PostgresRepo{}).scanEvent(rows)
		if err != nil {
			return nil, err
		}
		events = append(events, ev)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return events, nil
}
