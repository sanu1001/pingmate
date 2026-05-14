package services

import (
	"errors"

	"github.com/sanu1001/pingmate/internal/models"
	"github.com/sanu1001/pingmate/internal/repository"
)

// ReminderService defines operations available to handlers.
type ReminderService interface {
	Create(userID string, req models.CreateReminderRequest) (*models.Reminder, error)
	GetAll(userID string) ([]models.Reminder, error)
	GetByID(userID string, reminderID string) (*models.Reminder, error)
	Update(userID string, reminderID string, req models.UpdateReminderRequest) (*models.Reminder, error)
	Delete(userID string, reminderID string) error
}

// Common reminder errors — handlers map these to HTTP codes.
var (
	ErrReminderNotFound = errors.New("reminder not found")
)

type reminderService struct {
	reminderRepo repository.ReminderRepository
}

// NewReminderService is the constructor. main.go calls this.
func NewReminderService(reminderRepo repository.ReminderRepository) ReminderService {
	return &reminderService{reminderRepo: reminderRepo}
}

// ───────────────────────── Create ─────────────────────────

func (s *reminderService) Create(userID string, req models.CreateReminderRequest) (*models.Reminder, error) {
	reminder := &models.Reminder{
		UserID:      userID,
		Title:       req.Title,
		Description: req.Description,
		ScheduledAt: req.ScheduledAt,
		Recurrence:  req.Recurrence,
	}

	if err := s.reminderRepo.Create(reminder); err != nil {
		return nil, err
	}

	return reminder, nil
}

// ───────────────────────── GetAll ─────────────────────────

func (s *reminderService) GetAll(userID string) ([]models.Reminder, error) {
	reminders, err := s.reminderRepo.FindAll(userID)
	if err != nil {
		return nil, err
	}

	// Return empty slice instead of nil for clean JSON response ([])
	if reminders == nil {
		return []models.Reminder{}, nil
	}

	return reminders, nil
}

// ───────────────────────── GetByID ─────────────────────────

func (s *reminderService) GetByID(userID string, reminderID string) (*models.Reminder, error) {
	reminder, err := s.reminderRepo.FindByID(reminderID, userID)
	if err != nil {
		return nil, err
	}

	if reminder == nil {
		return nil, ErrReminderNotFound
	}

	return reminder, nil
}

// ───────────────────────── Update ─────────────────────────

func (s *reminderService) Update(userID string, reminderID string, req models.UpdateReminderRequest) (*models.Reminder, error) {
	// 1. Confirm reminder exists and belongs to this user
	reminder, err := s.reminderRepo.FindByID(reminderID, userID)
	if err != nil {
		return nil, err
	}
	if reminder == nil {
		return nil, ErrReminderNotFound
	}

	// 2. Merge — only overwrite fields the client actually sent
	if req.Title != "" {
		reminder.Title = req.Title
	}
	if req.Description != "" {
		reminder.Description = req.Description
	}
	if !req.ScheduledAt.IsZero() {
		reminder.ScheduledAt = req.ScheduledAt
	}
	if req.Recurrence != "" {
		reminder.Recurrence = req.Recurrence
	}

	// 3. Persist updated reminder
	if err := s.reminderRepo.Update(reminder); err != nil {
		return nil, err
	}

	return reminder, nil
}

// ───────────────────────── Delete ─────────────────────────

func (s *reminderService) Delete(userID string, reminderID string) error {
	// Confirm it exists and belongs to this user before deleting
	reminder, err := s.reminderRepo.FindByID(reminderID, userID)
	if err != nil {
		return err
	}
	if reminder == nil {
		return ErrReminderNotFound
	}

	return s.reminderRepo.Delete(reminderID, userID)
}
