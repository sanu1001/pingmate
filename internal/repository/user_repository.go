package repository

import (
	"database/sql"
	"errors"

	"github.com/sanu1001/pingmate/internal/models"
)

// UserRepository defines what operations are available.
// Services depend on this interface, never the concrete struct.
type UserRepository interface {
	CreateUser(user *models.User) error
	FindByEmail(email string) (*models.User, error)
	FindByID(id string) (*models.User, error)
}

// userRepo is the concrete PostgreSQL implementation.
// Unexported — only accessible via the interface.
type userRepo struct {
	db *sql.DB
}

// NewUserRepo is the constructor. main.go calls this
// and injects config.DB here.
func NewUserRepo(db *sql.DB) UserRepository {
	return &userRepo{db: db}
}

func (r *userRepo) CreateUser(user *models.User) error {
	query := `
		INSERT INTO users (email, password)
		VALUES ($1, $2)
		RETURNING id, created_at
	`

	return r.db.QueryRow(query, user.Email, user.Password).
		Scan(&user.ID, &user.CreatedAt)
}

func (r *userRepo) FindByEmail(email string) (*models.User, error) {
	query := `
		SELECT id, email, password, created_at
		FROM users
		WHERE email = $1
	`

	user := &models.User{}
	err := r.db.QueryRow(query, email).
		Scan(&user.ID, &user.Email, &user.Password, &user.CreatedAt)

	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}

	if err != nil {
		return nil, err
	}

	return user, nil
}

func (r *userRepo) FindByID(id string) (*models.User, error) {
	query := `
		SELECT id, email, created_at
		FROM users
		WHERE id = $1
	`

	user := &models.User{}
	err := r.db.QueryRow(query, id).
		Scan(&user.ID, &user.Email, &user.CreatedAt)

	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}

	if err != nil {
		return nil, err
	}

	return user, nil
}
