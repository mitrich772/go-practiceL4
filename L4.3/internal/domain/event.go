package domain

import "time"

// Event описывает событие календаря.
type Event struct {
	ID         int64      `json:"id"`
	UserID     int64      `json:"user_id"`
	Date       string     `json:"date"`
	Time       string     `json:"time"`
	Name       string     `json:"name"`
	RemindAt   *time.Time `json:"remind_at,omitempty"`
	RemindedAt *time.Time `json:"reminded_at,omitempty"`
	ArchivedAt *time.Time `json:"archived_at,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
}

// ReminderTask передаётся воркеру напоминаний через канал.
type ReminderTask struct {
	EventID  int64
	UserID   int64
	Name     string
	RemindAt time.Time
}

// IsArchived проверяет, перенесено ли событие в архив.
func (e Event) IsArchived() bool {
	return e.ArchivedAt != nil
}
