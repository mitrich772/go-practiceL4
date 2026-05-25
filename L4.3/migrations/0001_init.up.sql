CREATE TABLE IF NOT EXISTS events (
    id          BIGSERIAL PRIMARY KEY,
    user_id     BIGINT NOT NULL,
    event_date  DATE NOT NULL,
    event_time  TIME NOT NULL,
    name        TEXT NOT NULL,
    remind_at   TIMESTAMPTZ NULL,
    reminded_at TIMESTAMPTZ NULL,
    archived_at TIMESTAMPTZ NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_events_user_date_active
    ON events (user_id, event_date)
    WHERE archived_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_events_reminders
    ON events (remind_at)
    WHERE remind_at IS NOT NULL AND reminded_at IS NULL AND archived_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_events_archive
    ON events (event_date, event_time)
    WHERE archived_at IS NULL;
