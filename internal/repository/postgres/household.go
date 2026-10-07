package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/yovindoardana/geu-waste-api/internal/domain"
)

// HouseholdRepository implements database operations for households.
type HouseholdRepository struct {
	pool *pgxpool.Pool
}

// NewHouseholdRepository creates a new HouseholdRepository.
func NewHouseholdRepository(pool *pgxpool.Pool) *HouseholdRepository {
	return &HouseholdRepository{pool: pool}
}

// Create inserts a new household into the database.
func (r *HouseholdRepository) Create(ctx context.Context, h *domain.Household) error {
	query := `
		INSERT INTO households (id, owner_name, address, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5)
	`
	_, err := r.pool.Exec(ctx, query, h.ID, h.OwnerName, h.Address, h.CreatedAt, h.UpdatedAt)
	if err != nil {
		return fmt.Errorf("failed to insert household: %w", err)
	}
	return nil
}

// FindByID retrieves a household by its primary key UUID.
func (r *HouseholdRepository) FindByID(ctx context.Context, id string) (*domain.Household, error) {
	query := `
		SELECT id, owner_name, address, created_at, updated_at
		FROM households
		WHERE id = $1
	`
	var h domain.Household
	err := r.pool.QueryRow(ctx, query, id).Scan(
		&h.ID,
		&h.OwnerName,
		&h.Address,
		&h.CreatedAt,
		&h.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, fmt.Errorf("failed to query household by id: %w", err)
	}
	return &h, nil
}

// FindAll retrieves paginated households and the total count.
func (r *HouseholdRepository) FindAll(ctx context.Context, page, limit int) ([]domain.Household, int64, error) {
	countQuery := `SELECT COUNT(*) FROM households`
	var total int64
	if err := r.pool.QueryRow(ctx, countQuery).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("failed to count households: %w", err)
	}

	offset := (page - 1) * limit
	query := `
		SELECT id, owner_name, address, created_at, updated_at
		FROM households
		ORDER BY created_at DESC, id DESC
		LIMIT $1 OFFSET $2
	`

	rows, err := r.pool.Query(ctx, query, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to query households list: %w", err)
	}
	defer rows.Close()

	households := make([]domain.Household, 0)
	for rows.Next() {
		var h domain.Household
		if err := rows.Scan(&h.ID, &h.OwnerName, &h.Address, &h.CreatedAt, &h.UpdatedAt); err != nil {
			return nil, 0, fmt.Errorf("failed to scan household row: %w", err)
		}
		households = append(households, h)
	}

	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("rows iteration error: %w", err)
	}

	return households, total, nil
}

// Delete deletes a household if it exists and has no dependencies.
func (r *HouseholdRepository) Delete(ctx context.Context, id string) error {
	// Check existence first
	existsQuery := `SELECT EXISTS(SELECT 1 FROM households WHERE id = $1)`
	var exists bool
	if err := r.pool.QueryRow(ctx, existsQuery, id).Scan(&exists); err != nil {
		return fmt.Errorf("failed to check household existence: %w", err)
	}
	if !exists {
		return domain.ErrNotFound
	}

	// Check dependencies in waste_pickups or payments
	depQuery := `
		SELECT EXISTS(
			SELECT 1 FROM waste_pickups WHERE household_id = $1
			UNION
			SELECT 1 FROM payments WHERE household_id = $1
		)
	`
	var hasDeps bool
	if err := r.pool.QueryRow(ctx, depQuery, id).Scan(&hasDeps); err != nil {
		return fmt.Errorf("failed to check household dependencies: %w", err)
	}
	if hasDeps {
		return domain.ErrHouseholdHasDependencies
	}

	deleteQuery := `DELETE FROM households WHERE id = $1`
	tag, err := r.pool.Exec(ctx, deleteQuery, id)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23503" { // foreign_key_violation
			return domain.ErrHouseholdHasDependencies
		}
		return fmt.Errorf("failed to delete household: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}

	return nil
}
