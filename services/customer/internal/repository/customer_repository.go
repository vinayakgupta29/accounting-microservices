package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sync"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

var (
	ErrCustomerNotFound = errors.New("customer not found")
	ErrGSTINExists      = errors.New("customer with this GSTIN already exists for this tenant")
)

// Customer represents the database customer record.
type Customer struct {
	ID          int64
	TenantID    string
	CustID      string
	Name        string
	Address     string
	PhoneNumber string
	Email       string
	GSTIN       string
	DealerType  string
	PANCard     string
	Aadhaar     string
	CreatedAt   time.Time
}

// CustomerRepository defines customer data access operations.
type CustomerRepository interface {
	Create(ctx context.Context, c *Customer) error
	FindByCustID(ctx context.Context, tenantID, custID string) (*Customer, error)
	List(ctx context.Context, tenantID string, limit, offset int) ([]*Customer, int64, error)
	NextCustomerID(ctx context.Context, tenantID string) (string, error)
}

// PostgresCustomerRepository implements CustomerRepository with PostgreSQL.
type PostgresCustomerRepository struct {
	db *sql.DB
}

func NewPostgresCustomerRepository(db *sql.DB) *PostgresCustomerRepository {
	return &PostgresCustomerRepository{db: db}
}

func (r *PostgresCustomerRepository) NextCustomerID(ctx context.Context, tenantID string) (string, error) {
	var count int64
	query := `SELECT COUNT(*) FROM customers WHERE tenant_id = $1`
	if err := r.db.QueryRowContext(ctx, query, tenantID).Scan(&count); err != nil {
		return "", err
	}
	return fmt.Sprintf("cust_%03d", count+1), nil
}

func (r *PostgresCustomerRepository) Create(ctx context.Context, c *Customer) error {
	c.CreatedAt = time.Now()
	query := `INSERT INTO customers (tenant_id, cust_id, name, address, phone_number, email, gstin, dealer_type, pan_card, aadhaar, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11) RETURNING id`

	return r.db.QueryRowContext(ctx, query,
		c.TenantID, c.CustID, c.Name, c.Address, c.PhoneNumber, c.Email,
		c.GSTIN, c.DealerType, c.PANCard, c.Aadhaar, c.CreatedAt).Scan(&c.ID)
}

func (r *PostgresCustomerRepository) FindByCustID(ctx context.Context, tenantID, custID string) (*Customer, error) {
	query := `SELECT id, tenant_id, cust_id, name, address, phone_number, email, gstin, dealer_type, pan_card, aadhaar, created_at
		FROM customers WHERE tenant_id = $1 AND cust_id = $2 LIMIT 1`

	c := &Customer{}
	err := r.db.QueryRowContext(ctx, query, tenantID, custID).Scan(
		&c.ID, &c.TenantID, &c.CustID, &c.Name, &c.Address, &c.PhoneNumber,
		&c.Email, &c.GSTIN, &c.DealerType, &c.PANCard, &c.Aadhaar, &c.CreatedAt)

	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrCustomerNotFound
	}
	return c, err
}

func (r *PostgresCustomerRepository) List(ctx context.Context, tenantID string, limit, offset int) ([]*Customer, int64, error) {
	if limit <= 0 {
		limit = 100
	}

	var total int64
	countQuery := `SELECT COUNT(*) FROM customers WHERE tenant_id = $1`
	if err := r.db.QueryRowContext(ctx, countQuery, tenantID).Scan(&total); err != nil {
		return nil, 0, err
	}

	query := `SELECT id, tenant_id, cust_id, name, address, phone_number, email, gstin, dealer_type, pan_card, aadhaar, created_at
		FROM customers WHERE tenant_id = $1 ORDER BY id ASC LIMIT $2 OFFSET $3`

	rows, err := r.db.QueryContext(ctx, query, tenantID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var list []*Customer
	for rows.Next() {
		c := &Customer{}
		if err := rows.Scan(
			&c.ID, &c.TenantID, &c.CustID, &c.Name, &c.Address, &c.PhoneNumber,
			&c.Email, &c.GSTIN, &c.DealerType, &c.PANCard, &c.Aadhaar, &c.CreatedAt); err != nil {
			return nil, 0, err
		}
		list = append(list, c)
	}

	return list, total, nil
}

// MemoryCustomerRepository provides an in-memory fallback implementation.
type MemoryCustomerRepository struct {
	mu        sync.RWMutex
	customers map[string][]*Customer // tenant_id -> list of customers
	autoID    int64
}

func NewMemoryCustomerRepository() *MemoryCustomerRepository {
	return &MemoryCustomerRepository{
		customers: make(map[string][]*Customer),
	}
}

func (m *MemoryCustomerRepository) NextCustomerID(ctx context.Context, tenantID string) (string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	count := len(m.customers[tenantID])
	return fmt.Sprintf("cust_%03d", count+1), nil
}

func (m *MemoryCustomerRepository) Create(ctx context.Context, c *Customer) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, existing := range m.customers[c.TenantID] {
		if existing.GSTIN == c.GSTIN {
			return ErrGSTINExists
		}
	}

	m.autoID++
	c.ID = m.autoID
	c.CreatedAt = time.Now()

	m.customers[c.TenantID] = append(m.customers[c.TenantID], c)
	return nil
}

func (m *MemoryCustomerRepository) FindByCustID(ctx context.Context, tenantID, custID string) (*Customer, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, c := range m.customers[tenantID] {
		if c.CustID == custID {
			return c, nil
		}
	}
	return nil, ErrCustomerNotFound
}

func (m *MemoryCustomerRepository) List(ctx context.Context, tenantID string, limit, offset int) ([]*Customer, int64, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	all := m.customers[tenantID]
	total := int64(len(all))

	if offset >= len(all) {
		return []*Customer{}, total, nil
	}

	end := offset + limit
	if end > len(all) || limit <= 0 {
		end = len(all)
	}

	return all[offset:end], total, nil
}
