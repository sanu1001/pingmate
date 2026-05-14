package handlers

import (
	"errors"
	"net/http"
	"strings"

	"github.com/sanu1001/pingmate/internal/models"
	"github.com/sanu1001/pingmate/internal/services"

	"github.com/gin-gonic/gin"
)

type AuthHandler struct {
	authSvc services.AuthService
}

func NewAuthHandler(authSvc services.AuthService) *AuthHandler {
	return &AuthHandler{authSvc: authSvc}
}

// ───────────────────────── Register ─────────────────────────

// Register godoc
// @Summary     Register a new user
// @Tags        auth
// @Accept      json
// @Produce     json
// @Param       request body models.RegisterRequest true "Register payload"
// @Success     201 {object} models.AuthResponse
// @Failure     400 {object} map[string]string
// @Failure     409 {object} map[string]string
// @Router      /auth/register [post]
func (h *AuthHandler) Register(c *gin.Context) {
	var req models.RegisterRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	resp, err := h.authSvc.Register(req)
	if err != nil {
		if errors.Is(err, services.ErrEmailExists) {
			c.JSON(http.StatusConflict, gin.H{"error": "email already registered"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not register user"})
		return
	}

	c.JSON(http.StatusCreated, resp)
}

// ───────────────────────── Login ─────────────────────────

// Login godoc
// @Summary     Login with email and password
// @Tags        auth
// @Accept      json
// @Produce     json
// @Param       request body models.LoginRequest true "Login payload"
// @Success     200 {object} models.AuthResponse
// @Failure     400 {object} map[string]string
// @Failure     401 {object} map[string]string
// @Router      /auth/login [post]
func (h *AuthHandler) Login(c *gin.Context) {
	var req models.LoginRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	resp, err := h.authSvc.Login(req)
	if err != nil {
		if errors.Is(err, services.ErrInvalidCredentials) {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid email or password"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not login"})
		return
	}

	c.JSON(http.StatusOK, resp)
}

// ───────────────────────── Logout ─────────────────────────

// Logout godoc
// @Summary     Logout and invalidate token
// @Tags        auth
// @Security    BearerAuth
// @Produce     json
// @Success     200 {object} map[string]string
// @Failure     401 {object} map[string]string
// @Router      /auth/logout [post]
func (h *AuthHandler) Logout(c *gin.Context) {
	// Extract token from header — middleware already verified it
	authHeader := c.GetHeader("Authorization")
	tokenString := strings.TrimPrefix(authHeader, "Bearer ")

	if err := h.authSvc.Logout(tokenString); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "could not logout"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "logged out successfully"})
}
