package repository

import (
	"context"
	"database/sql"
	"errors"
	"sync"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

var (
	ErrProductNotFound     = errors.New("product not found")
	ErrInsufficientStock   = errors.New("insufficient stock available")
	ErrReservationNotFound = errors.New("reservation not found or already processed")
)

type Product struct {
	ID          int64
	TenantID    string
	ProductName string
	Quantity    int32
	UnitPrice   float64
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type ReservationItem struct {
	ProductID int64
	Quantity  int32
}

type StockReservation struct {
	ReservationID string
	TenantID      string
	Items         []ReservationItem
	Status        string // RESERVED, COMMITTED, RELEASED
	CreatedAt     time.Time
}

type InventoryRepository interface {
	Add(ctx context.Context, p *Product) error
	FindByID(ctx context.Context, tenantID string, id int64) (*Product, error)
	List(ctx context.Context, tenantID string) ([]*Product, error)
	ReserveStock(ctx context.Context, tenantID, reservationID string, items []ReservationItem) ([]*Product, error)
	CommitStock(ctx context.Context, tenantID, reservationID string) error
	ReleaseStock(ctx context.Context, tenantID, reservationID string) error
}

type PostgresInventoryRepository struct {
	db *sql.DB
}

func NewPostgresInventoryRepository(db *sql.DB) *PostgresInventoryRepository {
	return &PostgresInventoryRepository{db: db}
}

func (r *PostgresInventoryRepository) Add(ctx context.Context, p *Product) error {
	p.CreatedAt = time.Now()
	p.UpdatedAt = time.Now()

	query := `INSERT INTO inventory_items (tenant_id, product_name, quantity, unit_price, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6) RETURNING id`

	return r.db.QueryRowContext(ctx, query,
		p.TenantID, p.ProductName, p.Quantity, p.UnitPrice, p.CreatedAt, p.UpdatedAt).Scan(&p.ID)
}

func (r *PostgresInventoryRepository) FindByID(ctx context.Context, tenantID string, id int64) (*Product, error) {
	query := `SELECT id, tenant_id, product_name, quantity, unit_price, created_at, updated_at
		FROM inventory_items WHERE tenant_id = $1 AND id = $2 LIMIT 1`

	p := &Product{}
	err := r.db.QueryRowContext(ctx, query, tenantID, id).Scan(
		&p.ID, &p.TenantID, &p.ProductName, &p.Quantity, &p.UnitPrice, &p.CreatedAt, &p.UpdatedAt)

	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrProductNotFound
	}
	return p, err
}

func (r *PostgresInventoryRepository) List(ctx context.Context, tenantID string) ([]*Product, error) {
	query := `SELECT id, tenant_id, product_name, quantity, unit_price, created_at, updated_at
		FROM inventory_items WHERE tenant_id = $1 ORDER BY id ASC`

	rows, err := r.db.QueryContext(ctx, query, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []*Product
	for rows.Next() {
		p := &Product{}
		if err := rows.Scan(
			&p.ID, &p.TenantID, &p.ProductName, &p.Quantity, &p.UnitPrice, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, err
		}
		list = append(list, p)
	}
	return list, nil
}

func (r *PostgresInventoryRepository) ReserveStock(ctx context.Context, tenantID, reservationID string, items []ReservationItem) ([]*Product, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	var reservedProducts []*Product

	for _, item := range items {
		var available int32
		var p Product
		query := `SELECT id, tenant_id, product_name, quantity, unit_price, created_at, updated_at
			FROM inventory_items WHERE tenant_id = $1 AND id = $2 FOR UPDATE`

		err := tx.QueryRowContext(ctx, query, tenantID, item.ProductID).Scan(
			&p.ID, &p.TenantID, &p.ProductName, &available, &p.UnitPrice, &p.CreatedAt, &p.UpdatedAt)

		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrProductNotFound
		}
		if err != nil {
			return nil, err
		}

		if available < item.Quantity {
			return nil, ErrInsufficientStock
		}

		// Deduct stock for reservation
		updateQuery := `UPDATE inventory_items SET quantity = quantity - $1, updated_at = NOW() WHERE id = $2`
		if _, err := tx.ExecContext(ctx, updateQuery, item.Quantity, item.ProductID); err != nil {
			return nil, err
		}

		// Record reservation
		resQuery := `INSERT INTO stock_reservations (reservation_id, tenant_id, product_id, quantity, status, created_at)
			VALUES ($1, $2, $3, $4, 'RESERVED', NOW())`
		if _, err := tx.ExecContext(ctx, resQuery, reservationID, tenantID, item.ProductID, item.Quantity); err != nil {
			return nil, err
		}

		p.Quantity = available - item.Quantity
		reservedProducts = append(reservedProducts, &p)
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return reservedProducts, nil
}

func (r *PostgresInventoryRepository) CommitStock(ctx context.Context, tenantID, reservationID string) error {
	query := `UPDATE stock_reservations SET status = 'COMMITTED' WHERE tenant_id = $1 AND reservation_id = $2 AND status = 'RESERVED'`
	res, err := r.db.ExecContext(ctx, query, tenantID, reservationID)
	if err != nil {
		return err
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return ErrReservationNotFound
	}
	return nil
}

func (r *PostgresInventoryRepository) ReleaseStock(ctx context.Context, tenantID, reservationID string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	query := `SELECT product_id, quantity FROM stock_reservations WHERE tenant_id = $1 AND reservation_id = $2 AND status = 'RESERVED'`
	rows, err := tx.QueryContext(ctx, query, tenantID, reservationID)
	if err != nil {
		return err
	}
	defer rows.Close()

	type resItem struct {
		productID int64
		quantity  int32
	}
	var items []resItem
	for rows.Next() {
		var item resItem
		if err := rows.Scan(&item.productID, &item.quantity); err != nil {
			return err
		}
		items = append(items, item)
	}

	if len(items) == 0 {
		return ErrReservationNotFound
	}

	for _, it := range items {
		restoreQuery := `UPDATE inventory_items SET quantity = quantity + $1, updated_at = NOW() WHERE id = $2`
		if _, err := tx.ExecContext(ctx, restoreQuery, it.quantity, it.productID); err != nil {
			return err
		}
	}

	statusQuery := `UPDATE stock_reservations SET status = 'RELEASED' WHERE tenant_id = $1 AND reservation_id = $2`
	if _, err := tx.ExecContext(ctx, statusQuery, tenantID, reservationID); err != nil {
		return err
	}

	return tx.Commit()
}

// MemoryInventoryRepository provides an in-memory fallback implementation.
type MemoryInventoryRepository struct {
	mu           sync.RWMutex
	products     map[string]map[int64]*Product          // tenant_id -> id -> Product
	reservations map[string]map[string]*StockReservation // tenant_id -> res_id -> Reservation
	autoID       int64
}

func NewMemoryInventoryRepository() *MemoryInventoryRepository {
	return &MemoryInventoryRepository{
		products:     make(map[string]map[int64]*Product),
		reservations: make(map[string]map[string]*StockReservation),
	}
}

func (m *MemoryInventoryRepository) Add(ctx context.Context, p *Product) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.products[p.TenantID] == nil {
		m.products[p.TenantID] = make(map[int64]*Product)
	}

	m.autoID++
	p.ID = m.autoID
	p.CreatedAt = time.Now()
	p.UpdatedAt = time.Now()

	m.products[p.TenantID][p.ID] = p
	return nil
}

func (m *MemoryInventoryRepository) FindByID(ctx context.Context, tenantID string, id int64) (*Product, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	tenantMap, ok := m.products[tenantID]
	if !ok {
		return nil, ErrProductNotFound
	}
	p, ok := tenantMap[id]
	if !ok {
		return nil, ErrProductNotFound
	}
	return p, nil
}

func (m *MemoryInventoryRepository) List(ctx context.Context, tenantID string) ([]*Product, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	tenantMap := m.products[tenantID]
	var list []*Product
	for _, p := range tenantMap {
		list = append(list, p)
	}
	return list, nil
}

func (m *MemoryInventoryRepository) ReserveStock(ctx context.Context, tenantID, reservationID string, items []ReservationItem) ([]*Product, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	tenantMap, ok := m.products[tenantID]
	if !ok {
		return nil, ErrProductNotFound
	}

	// Validation pass
	for _, it := range items {
		p, exists := tenantMap[it.ProductID]
		if !exists {
			return nil, ErrProductNotFound
		}
		if p.Quantity < it.Quantity {
			return nil, ErrInsufficientStock
		}
	}

	// Execution pass
	var reservedProducts []*Product
	for _, it := range items {
		p := tenantMap[it.ProductID]
		p.Quantity -= it.Quantity
		p.UpdatedAt = time.Now()
		reservedProducts = append(reservedProducts, p)
	}

	if m.reservations[tenantID] == nil {
		m.reservations[tenantID] = make(map[string]*StockReservation)
	}
	m.reservations[tenantID][reservationID] = &StockReservation{
		ReservationID: reservationID,
		TenantID:      tenantID,
		Items:         items,
		Status:        "RESERVED",
		CreatedAt:     time.Now(),
	}

	return reservedProducts, nil
}

func (m *MemoryInventoryRepository) CommitStock(ctx context.Context, tenantID, reservationID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	resMap, ok := m.reservations[tenantID]
	if !ok {
		return ErrReservationNotFound
	}
	res, ok := resMap[reservationID]
	if !ok || res.Status != "RESERVED" {
		return ErrReservationNotFound
	}
	res.Status = "COMMITTED"
	return nil
}

func (m *MemoryInventoryRepository) ReleaseStock(ctx context.Context, tenantID, reservationID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	resMap, ok := m.reservations[tenantID]
	if !ok {
		return ErrReservationNotFound
	}
	res, ok := resMap[reservationID]
	if !ok || res.Status != "RESERVED" {
		return ErrReservationNotFound
	}

	tenantMap := m.products[tenantID]
	for _, it := range res.Items {
		if p, ok := tenantMap[it.ProductID]; ok {
			p.Quantity += it.Quantity
			p.UpdatedAt = time.Now()
		}
	}
	res.Status = "RELEASED"
	return nil
}
