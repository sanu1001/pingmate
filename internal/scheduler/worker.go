package scheduler

import (
	"log"
	"time"

	"github.com/sanu1001/pingmate/internal/models"
	"github.com/sanu1001/pingmate/internal/repository"
)

// Scheduler runs the background reminder-firing loop.
type Scheduler struct {
	reminderRepo repository.ReminderRepository
	interval     time.Duration
}

// NewScheduler is the constructor. main.go calls this.
func NewScheduler(reminderRepo repository.ReminderRepository, interval time.Duration) *Scheduler {
	return &Scheduler{
		reminderRepo: reminderRepo,
		interval:     interval,
	}
}

// Start runs the scheduler loop forever in a goroutine.
// main.go calls: go scheduler.Start()
func (s *Scheduler) Start() {
	log.Printf("⏰ Scheduler started — polling every %s", s.interval)

	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	// Run once immediately on boot, then on every tick
	s.tick()

	for range ticker.C {
		s.tick()
	}
}

// tick is one iteration of the scheduler loop.
func (s *Scheduler) tick() {
	dueReminders, err := s.reminderRepo.FindDueReminders()
	if err != nil {
		log.Printf("⚠️  Scheduler: failed to fetch due reminders: %v", err)
		return
	}

	if len(dueReminders) == 0 {
		return
	}

	log.Printf("⏰ Scheduler: processing %d due reminder(s)", len(dueReminders))

	for _, reminder := range dueReminders {
		s.processReminder(reminder)
	}
}

// processReminder handles a single due reminder:
//   - logs the trigger to console
//   - writes a notification_log entry
//   - advances scheduled_at (recurring) OR deactivates (one-shot)
func (s *Scheduler) processReminder(reminder models.Reminder) {
	log.Printf("🔔 TRIGGERED: \"%s\" for user %s", reminder.Title, reminder.UserID)

	// Write to notification_logs — audit trail
	logEntry := &models.NotificationLog{
		ReminderID: reminder.ID,
		Status:     models.LogStatusSent,
	}
	if err := s.reminderRepo.CreateLog(logEntry); err != nil {
		log.Printf("⚠️  Scheduler: failed to log reminder %s: %v", reminder.ID, err)
	}

	// Decide what to do with the reminder going forward
	switch reminder.Recurrence {
	case models.RecurrenceNone:
		// One-shot — deactivate so it never fires again
		reminder.IsActive = false

	case models.RecurrenceDaily:
		reminder.ScheduledAt = reminder.ScheduledAt.AddDate(0, 0, 1)

	case models.RecurrenceWeekly:
		reminder.ScheduledAt = reminder.ScheduledAt.AddDate(0, 0, 7)

	case models.RecurrenceMonthly:
		reminder.ScheduledAt = reminder.ScheduledAt.AddDate(0, 1, 0)
	}

	if err := s.reminderRepo.Update(&reminder); err != nil {
		log.Printf("⚠️  Scheduler: failed to update reminder %s: %v", reminder.ID, err)
	}
}
