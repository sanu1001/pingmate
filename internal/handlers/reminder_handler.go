package handlers

import (
	"errors"
	"net/http"

	"github.com/sanu1001/pingmate/internal/models"
	"github.com/sanu1001/pingmate/internal/services"

	"github.com/gin-gonic/gin"
)

type ReminderHandler struct {
	reminderSvc services.ReminderService
}

func NewReminderHandler(reminderSvc services.ReminderService) *ReminderHandler {
	return &ReminderHandler{reminderSvc: reminderSvc}
}

// ───────────────────────── Create ─────────────────────────

// Create godoc
// @Summary     Create a new reminder
// @Tags        reminders
// @Security    BearerAuth
// @Accept      json
// @Produce     json
// @Param       request body models.CreateReminderRequest true "Reminder payload"
// @Success     201 {object} models.Reminder
// @Failure     400 {object} map[string]string
// @Failure     401 {object} map[string]string
// @Router      /reminders [post]
func (h *ReminderHandler) Create(c *gin.Context) {
	userID := c.GetString("user_id")

	var req models.CreateReminderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	reminder, err := h.reminderSvc.Create(userID, req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not create reminder"})
		return
	}

	c.JSON(http.StatusCreated, reminder)
}

// ───────────────────────── GetAll ─────────────────────────

// GetAll godoc
// @Summary     List all reminders for the authenticated user
// @Tags        reminders
// @Security    BearerAuth
// @Produce     json
// @Success     200 {array} models.Reminder
// @Failure     401 {object} map[string]string
// @Router      /reminders [get]
func (h *ReminderHandler) GetAll(c *gin.Context) {
	userID := c.GetString("user_id")

	reminders, err := h.reminderSvc.GetAll(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not fetch reminders"})
		return
	}

	c.JSON(http.StatusOK, reminders)
}

// ───────────────────────── GetByID ─────────────────────────

// GetByID godoc
// @Summary     Get a single reminder by ID
// @Tags        reminders
// @Security    BearerAuth
// @Produce     json
// @Param       id path string true "Reminder ID"
// @Success     200 {object} models.Reminder
// @Failure     401 {object} map[string]string
// @Failure     404 {object} map[string]string
// @Router      /reminders/{id} [get]
func (h *ReminderHandler) GetByID(c *gin.Context) {
	userID := c.GetString("user_id")
	reminderID := c.Param("id")

	reminder, err := h.reminderSvc.GetByID(userID, reminderID)
	if err != nil {
		if errors.Is(err, services.ErrReminderNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "reminder not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not fetch reminder"})
		return
	}

	c.JSON(http.StatusOK, reminder)
}

// ───────────────────────── Update ─────────────────────────

// Update godoc
// @Summary     Update a reminder
// @Tags        reminders
// @Security    BearerAuth
// @Accept      json
// @Produce     json
// @Param       id path string true "Reminder ID"
// @Param       request body models.UpdateReminderRequest true "Update payload"
// @Success     200 {object} models.Reminder
// @Failure     400 {object} map[string]string
// @Failure     401 {object} map[string]string
// @Failure     404 {object} map[string]string
// @Router      /reminders/{id} [put]
func (h *ReminderHandler) Update(c *gin.Context) {
	userID := c.GetString("user_id")
	reminderID := c.Param("id")

	var req models.UpdateReminderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	reminder, err := h.reminderSvc.Update(userID, reminderID, req)
	if err != nil {
		if errors.Is(err, services.ErrReminderNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "reminder not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not update reminder"})
		return
	}

	c.JSON(http.StatusOK, reminder)
}

// ───────────────────────── Delete ─────────────────────────

// Delete godoc
// @Summary     Delete a reminder
// @Tags        reminders
// @Security    BearerAuth
// @Produce     json
// @Param       id path string true "Reminder ID"
// @Success     200 {object} map[string]string
// @Failure     401 {object} map[string]string
// @Failure     404 {object} map[string]string
// @Router      /reminders/{id} [delete]
func (h *ReminderHandler) Delete(c *gin.Context) {
	userID := c.GetString("user_id")
	reminderID := c.Param("id")

	if err := h.reminderSvc.Delete(userID, reminderID); err != nil {
		if errors.Is(err, services.ErrReminderNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "reminder not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not delete reminder"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "reminder deleted successfully"})
}
