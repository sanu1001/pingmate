package models

import "time"

type RecurrenceType string
type LogStatus string

const (
	RecurrenceNone    RecurrenceType = "none"
	RecurrenceDaily   RecurrenceType = "daily"
	RecurrenceWeekly  RecurrenceType = "weekly"
	RecurrenceMonthly RecurrenceType = "monthly"
)

const (
	LogStatusSent   LogStatus = "sent"
	LogStatusFailed LogStatus = "failed"
)

type Reminder struct {
	ID          string         `json:"id"`
	UserID      string         `json:"user_id"`
	Title       string         `json:"title"`
	Description string         `json:"description"`
	ScheduledAt time.Time      `json:"scheduled_at"`
	Recurrence  RecurrenceType `json:"recurrence"`
	IsActive    bool           `json:"is_active"`
	CreatedAt   time.Time      `json:"created_at"`
}

type NotificationLog struct {
	ID          string    `json:"id"`
	ReminderID  string    `json:"reminder_id"`
	TriggeredAt time.Time `json:"triggered_at"`
	Status      LogStatus `json:"status"`
}

type CreateReminderRequest struct {
	Title       string         `json:"title" binding:"required"`
	Description string         `json:"description"`
	ScheduledAt time.Time      `json:"scheduled_at" binding:"required"`
	Recurrence  RecurrenceType `json:"recurrence" binding:"required,oneof=none daily weekly monthly"`
}

type UpdateReminderRequest struct {
	Title       string         `json:"title"`
	Description string         `json:"description"`
	ScheduledAt time.Time      `json:"scheduled_at"`
	Recurrence  RecurrenceType `json:"recurrence" binding:"omitempty,oneof=none daily weekly monthly"`
}
