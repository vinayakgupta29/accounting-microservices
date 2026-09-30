package repository

import (
	"context"
	"database/sql"
	"errors"
	"sync"
	"time"

	"github.com/google/uuid"
	_ "github.com/jackc/pgx/v5/stdlib"
)

var (
	ErrUserNotFound      = errors.New("user not found")
	ErrUsernameExists    = errors.New("username already exists")
	ErrEmailExists       = errors.New("email already registered")
)

// User represents the database user model.
type User struct {
	ID           string
	Name         string
	Username     string
	Email        string
	PasswordHash string
	GSTIN        string
	PANCard      string
	Aadhaar      string
	Phone        string
	Address      string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// UserRepository interface defines persistence operations for user accounts.
type UserRepository interface {
	Create(ctx context.Context, u *User) error
	FindByUsername(ctx context.Context, username string) (*User, error)
	FindByID(ctx context.Context, id string) (*User, error)
}

// PostgresUserRepository implements UserRepository with PostgreSQL.
type PostgresUserRepository struct {
	db *sql.DB
}

// NewPostgresUserRepository initializes a PostgreSQL user repository.
func NewPostgresUserRepository(db *sql.DB) *PostgresUserRepository {
	return &PostgresUserRepository{db: db}
}

func (r *PostgresUserRepository) Create(ctx context.Context, u *User) error {
	if u.ID == "" {
		u.ID = uuid.New().String()
	}
	u.CreatedAt = time.Now()
	u.UpdatedAt = time.Now()

	query := `INSERT INTO users (id, name, username, email, password_hash, gstin, pan_card, aadhaar, phone, address, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)`

	_, err := r.db.ExecContext(ctx, query,
		u.ID, u.Name, u.Username, u.Email, u.PasswordHash, u.GSTIN,
		u.PANCard, u.Aadhaar, u.Phone, u.Address, u.CreatedAt, u.UpdatedAt)

	return err
}

func (r *PostgresUserRepository) FindByUsername(ctx context.Context, username string) (*User, error) {
	query := `SELECT id, name, username, email, password_hash, gstin, pan_card, aadhaar, phone, address, created_at, updated_at
		FROM users WHERE username = $1 LIMIT 1`

	u := &User{}
	err := r.db.QueryRowContext(ctx, query, username).Scan(
		&u.ID, &u.Name, &u.Username, &u.Email, &u.PasswordHash, &u.GSTIN,
		&u.PANCard, &u.Aadhaar, &u.Phone, &u.Address, &u.CreatedAt, &u.UpdatedAt)

	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrUserNotFound
	}
	return u, err
}

func (r *PostgresUserRepository) FindByID(ctx context.Context, id string) (*User, error) {
	query := `SELECT id, name, username, email, password_hash, gstin, pan_card, aadhaar, phone, address, created_at, updated_at
		FROM users WHERE id = $1 LIMIT 1`

	u := &User{}
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&u.ID, &u.Name, &u.Username, &u.Email, &u.PasswordHash, &u.GSTIN,
		&u.PANCard, &u.Aadhaar, &u.Phone, &u.Address, &u.CreatedAt, &u.UpdatedAt)

	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrUserNotFound
	}
	return u, err
}

// MemoryUserRepository provides an in-memory thread-safe fallback repository.
type MemoryUserRepository struct {
	mu    sync.RWMutex
	users map[string]*User // username -> User
	byID  map[string]*User // id -> User
}

func NewMemoryUserRepository() *MemoryUserRepository {
	return &MemoryUserRepository{
		users: make(map[string]*User),
		byID:  make(map[string]*User),
	}
}

func (m *MemoryUserRepository) Create(ctx context.Context, u *User) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.users[u.Username]; exists {
		return ErrUsernameExists
	}
	if u.ID == "" {
		u.ID = uuid.New().String()
	}
	u.CreatedAt = time.Now()
	u.UpdatedAt = time.Now()

	m.users[u.Username] = u
	m.byID[u.ID] = u
	return nil
}

func (m *MemoryUserRepository) FindByUsername(ctx context.Context, username string) (*User, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	u, exists := m.users[username]
	if !exists {
		return nil, ErrUserNotFound
	}
	return u, nil
}

func (m *MemoryUserRepository) FindByID(ctx context.Context, id string) (*User, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	u, exists := m.byID[id]
	if !exists {
		return nil, ErrUserNotFound
	}
	return u, nil
}
