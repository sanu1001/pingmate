package repository

import (
	"database/sql"
	"errors"
	"strings"

	"github.com/sanu1001/pingmate/internal/models"
)

// ReminderRepository defines all reminder + log operations.
// Scheduler and service both depend on this interface.
type ReminderRepository interface {
	Create(reminder *models.Reminder) error
	FindAll(userID string) ([]models.Reminder, error)
	FindByID(id string, userID string) (*models.Reminder, error)
	Update(reminder *models.Reminder) error
	Delete(id string, userID string) error
	FindDueReminders() ([]models.Reminder, error)
	CreateLog(log *models.NotificationLog) error
}

type reminderRepo struct {
	db *sql.DB
}

func NewReminderRepo(db *sql.DB) ReminderRepository {
	return &reminderRepo{db: db}
}

func (r *reminderRepo) Create(reminder *models.Reminder) error {
	query := `
		INSERT INTO reminders (user_id, title, description, scheduled_at, recurrence)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, is_active, created_at
	`

	return r.db.QueryRow(
		query,
		reminder.UserID,
		reminder.Title,
		reminder.Description,
		reminder.ScheduledAt,
		reminder.Recurrence,
	).Scan(&reminder.ID, &reminder.IsActive, &reminder.CreatedAt)
}

func (r *reminderRepo) FindAll(userID string) ([]models.Reminder, error) {
	query := `
		SELECT id, user_id, title, description, scheduled_at, recurrence, is_active, created_at
		FROM reminders
		WHERE user_id = $1
		ORDER BY scheduled_at ASC
	`

	rows, err := r.db.Query(query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var reminders []models.Reminder

	for rows.Next() {
		var rem models.Reminder
		err := rows.Scan(
			&rem.ID,
			&rem.UserID,
			&rem.Title,
			&rem.Description,
			&rem.ScheduledAt,
			&rem.Recurrence,
			&rem.IsActive,
			&rem.CreatedAt,
		)
		if err != nil {
			return nil, err
		}
		reminders = append(reminders, rem)
	}

	return reminders, nil
}

func (r *reminderRepo) FindByID(id string, userID string) (*models.Reminder, error) {
	query := `
		SELECT id, user_id, title, description, scheduled_at, recurrence, is_active, created_at
		FROM reminders
		WHERE id = $1 AND user_id = $2
	`

	rem := &models.Reminder{}
	err := r.db.QueryRow(query, id, userID).Scan(
		&rem.ID,
		&rem.UserID,
		&rem.Title,
		&rem.Description,
		&rem.ScheduledAt,
		&rem.Recurrence,
		&rem.IsActive,
		&rem.CreatedAt,
	)

	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}

	// Postgres rejects non-UUID strings with this error code (22P02)
	// Treat invalid UUID format as "not found" not a server error
	if err != nil && strings.Contains(err.Error(), "invalid input syntax for type uuid") {
		return nil, nil
	}

	if err != nil {
		return nil, err
	}

	return rem, nil
}

func (r *reminderRepo) Update(reminder *models.Reminder) error {
	query := `
		UPDATE reminders
		SET title = $1,
		    description = $2,
		    scheduled_at = $3,
		    recurrence = $4,
		    is_active = $5
		WHERE id = $6 AND user_id = $7
	`

	_, err := r.db.Exec(
		query,
		reminder.Title,
		reminder.Description,
		reminder.ScheduledAt,
		reminder.Recurrence,
		reminder.IsActive,
		reminder.ID,
		reminder.UserID,
	)

	return err
}

func (r *reminderRepo) Delete(id string, userID string) error {
	query := `
		DELETE FROM reminders
		WHERE id = $1 AND user_id = $2
	`

	result, err := r.db.Exec(query, id, userID)
	if err != nil {
		if strings.Contains(err.Error(), "invalid input syntax for type uuid") {
			return nil
		}
		return err
	}

	_ = result
	return nil
}

func (r *reminderRepo) FindDueReminders() ([]models.Reminder, error) {
	query := `
		SELECT id, user_id, title, description, scheduled_at, recurrence, is_active, created_at
		FROM reminders
		WHERE scheduled_at <= NOW()
		AND is_active = TRUE
	`

	rows, err := r.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var reminders []models.Reminder

	for rows.Next() {
		var rem models.Reminder
		err := rows.Scan(
			&rem.ID,
			&rem.UserID,
			&rem.Title,
			&rem.Description,
			&rem.ScheduledAt,
			&rem.Recurrence,
			&rem.IsActive,
			&rem.CreatedAt,
		)
		if err != nil {
			return nil, err
		}
		reminders = append(reminders, rem)
	}

	return reminders, nil
}

func (r *reminderRepo) CreateLog(log *models.NotificationLog) error {
	query := `
		INSERT INTO notification_logs (reminder_id, status)
		VALUES ($1, $2)
		RETURNING id, triggered_at
	`

	return r.db.QueryRow(query, log.ReminderID, log.Status).
		Scan(&log.ID, &log.TriggeredAt)
}
