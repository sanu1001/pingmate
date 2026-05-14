CREATE TYPE recurrence_type AS ENUM ('none', 'daily', 'weekly', 'monthly');

CREATE TABLE IF NOT EXISTS reminders (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id      UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    title        TEXT NOT NULL,
    description  TEXT,
    scheduled_at TIMESTAMPTZ NOT NULL,
    recurrence   recurrence_type NOT NULL DEFAULT 'none',
    is_active    BOOLEAN NOT NULL DEFAULT TRUE,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_reminders_user_id ON reminders(user_id);
CREATE INDEX idx_reminders_scheduled_at ON reminders(scheduled_at) WHERE is_active = TRUE;