package services

import (
	"context"
	"errors"
	"time"

	"github.com/sanu1001/pingmate/config"
	"github.com/sanu1001/pingmate/internal/models"
	"github.com/sanu1001/pingmate/internal/repository"

	"github.com/golang-jwt/jwt/v5"
	"github.com/redis/go-redis/v9"
	"golang.org/x/crypto/bcrypt"
)

// AuthService defines the operations a handler can call.
type AuthService interface {
	Register(req models.RegisterRequest) (*models.AuthResponse, error)
	Login(req models.LoginRequest) (*models.AuthResponse, error)
	Logout(tokenString string) error
	IsBlacklisted(tokenString string) (bool, error)
}

// authService is the concrete implementation.
type authService struct {
	userRepo repository.UserRepository
	redis    *redis.Client
}

// Common service errors — handler layer maps these to HTTP codes.
var (
	ErrEmailExists        = errors.New("email already registered")
	ErrInvalidCredentials = errors.New("invalid email or password")
	ErrInvalidToken       = errors.New("invalid or expired token")
)

// JWTClaims is what we embed inside every token.
type JWTClaims struct {
	UserID string `json:"user_id"`
	Email  string `json:"email"`
	jwt.RegisteredClaims
}

// NewAuthService is the constructor. main.go calls this.
func NewAuthService(userRepo repository.UserRepository, rdb *redis.Client) AuthService {
	return &authService{
		userRepo: userRepo,
		redis:    rdb,
	}
}

// ───────────────────────── Register ─────────────────────────

func (s *authService) Register(req models.RegisterRequest) (*models.AuthResponse, error) {
	// 1. Check if email is already taken
	existing, err := s.userRepo.FindByEmail(req.Email)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return nil, ErrEmailExists
	}

	// 2. Hash the password (bcrypt with cost factor 12)
	hashed, err := bcrypt.GenerateFromPassword([]byte(req.Password), 12)
	if err != nil {
		return nil, err
	}

	// 3. Build user and persist
	user := &models.User{
		Email:    req.Email,
		Password: string(hashed),
	}
	if err := s.userRepo.CreateUser(user); err != nil {
		return nil, err
	}

	// 4. Generate JWT so user is logged in immediately after register
	token, err := s.generateToken(user)
	if err != nil {
		return nil, err
	}

	// 5. Wipe password before sending response
	user.Password = ""

	return &models.AuthResponse{
		Token: token,
		User:  *user,
	}, nil
}

// ───────────────────────── Login ─────────────────────────

func (s *authService) Login(req models.LoginRequest) (*models.AuthResponse, error) {
	// 1. Find user
	user, err := s.userRepo.FindByEmail(req.Email)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, ErrInvalidCredentials
	}

	// 2. Compare password against stored hash
	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)); err != nil {
		return nil, ErrInvalidCredentials
	}

	// 3. Generate JWT
	token, err := s.generateToken(user)
	if err != nil {
		return nil, err
	}

	user.Password = ""

	return &models.AuthResponse{
		Token: token,
		User:  *user,
	}, nil
}

// ───────────────────────── Logout ─────────────────────────

func (s *authService) Logout(tokenString string) error {
	// 1. Parse token to get its expiry
	claims := &JWTClaims{}
	_, err := jwt.ParseWithClaims(tokenString, claims, func(t *jwt.Token) (any, error) {
		return []byte(config.App.JWTSecret), nil
	})
	if err != nil {
		return ErrInvalidToken
	}

	// 2. Calculate how long until this token naturally expires
	remaining := time.Until(claims.ExpiresAt.Time)
	if remaining <= 0 {
		// Already expired — nothing to blacklist
		return nil
	}

	// 3. Store in Redis with TTL equal to remaining lifetime.
	//    Once it expires naturally, Redis auto-removes it.
	ctx := context.Background()
	key := "blacklist:" + tokenString
	return s.redis.Set(ctx, key, "1", remaining).Err()
}

// ───────────────────── Blacklist Check ─────────────────────

// Called by the auth middleware on every protected request.
func (s *authService) IsBlacklisted(tokenString string) (bool, error) {
	ctx := context.Background()
	key := "blacklist:" + tokenString

	_, err := s.redis.Get(ctx, key).Result()
	if errors.Is(err, redis.Nil) {
		// Key not found = token is clean
		return false, nil
	}
	if err != nil {
		return false, err
	}

	// Key found = token is blacklisted
	return true, nil
}

// ───────────────────── Token Generation ─────────────────────

func (s *authService) generateToken(user *models.User) (string, error) {
	expiresAt := time.Now().Add(time.Duration(config.App.JWTExpiryHours) * time.Hour)

	claims := JWTClaims{
		UserID: user.ID,
		Email:  user.Email,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(config.App.JWTSecret))
}
