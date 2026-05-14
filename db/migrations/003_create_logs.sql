CREATE TYPE log_status AS ENUM ('sent', 'failed');

CREATE TABLE IF NOT EXISTS notification_logs (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    reminder_id  UUID NOT NULL REFERENCES reminders(id) ON DELETE CASCADE,
    triggered_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    status       log_status NOT NULL
);

CREATE INDEX idx_logs_reminder_id ON notification_logs(reminder_id);