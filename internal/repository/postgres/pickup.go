package postgres

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/yovindoardana/geu-waste-api/internal/domain"
)

// PickupRepository implements database operations for waste pickups.
type PickupRepository struct {
	pool *pgxpool.Pool
}

// NewPickupRepository creates a new PickupRepository.
func NewPickupRepository(pool *pgxpool.Pool) *PickupRepository {
	return &PickupRepository{pool: pool}
}

// CreateWithHouseholdLock creates a pickup within a transaction, locking household and validating no pending payment.
func (r *PickupRepository) CreateWithHouseholdLock(ctx context.Context, p *domain.Pickup) error {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	// 1. Lock household row and verify existence
	var householdExists bool
	lockQuery := `SELECT EXISTS(SELECT 1 FROM households WHERE id = $1 FOR UPDATE)`
	if err := tx.QueryRow(ctx, lockQuery, p.HouseholdID).Scan(&householdExists); err != nil {
		return fmt.Errorf("failed to lock household: %w", err)
	}
	if !householdExists {
		return domain.ErrNotFound
	}

	// 2. Check if household has pending payments (BR01)
	pendingQuery := `SELECT EXISTS(SELECT 1 FROM payments WHERE household_id = $1 AND status = 'pending')`
	var hasPending bool
	if err := tx.QueryRow(ctx, pendingQuery, p.HouseholdID).Scan(&hasPending); err != nil {
		return fmt.Errorf("failed to check pending payments: %w", err)
	}
	if hasPending {
		return domain.ErrHouseholdPendingPayment
	}

	// 3. Insert pickup record
	insertQuery := `
		INSERT INTO waste_pickups (id, household_id, type, status, pickup_date, safety_check, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`
	_, err = tx.Exec(ctx, insertQuery, p.ID, p.HouseholdID, p.Type, p.Status, p.PickupDate, p.SafetyCheck, p.CreatedAt, p.UpdatedAt)
	if err != nil {
		return fmt.Errorf("failed to insert pickup: %w", err)
	}

	return tx.Commit(ctx)
}

// FindByID retrieves a pickup by primary key UUID.
func (r *PickupRepository) FindByID(ctx context.Context, id string) (*domain.Pickup, error) {
	query := `
		SELECT id, household_id, type, status, pickup_date, safety_check, created_at, updated_at
		FROM waste_pickups
		WHERE id = $1
	`
	var p domain.Pickup
	err := r.pool.QueryRow(ctx, query, id).Scan(
		&p.ID,
		&p.HouseholdID,
		&p.Type,
		&p.Status,
		&p.PickupDate,
		&p.SafetyCheck,
		&p.CreatedAt,
		&p.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, fmt.Errorf("failed to query pickup: %w", err)
	}
	return &p, nil
}

// FindAll retrieves filtered and paginated pickups.
func (r *PickupRepository) FindAll(ctx context.Context, filter domain.PickupFilter) ([]domain.Pickup, int64, error) {
	whereClauses := make([]string, 0)
	args := make([]any, 0)
	argIdx := 1

	if filter.Status != nil && *filter.Status != "" {
		whereClauses = append(whereClauses, fmt.Sprintf("status = $%d", argIdx))
		args = append(args, *filter.Status)
		argIdx++
	}

	if filter.HouseholdID != nil && *filter.HouseholdID != "" {
		whereClauses = append(whereClauses, fmt.Sprintf("household_id = $%d", argIdx))
		args = append(args, *filter.HouseholdID)
		argIdx++
	}

	whereSQL := ""
	if len(whereClauses) > 0 {
		whereSQL = "WHERE " + strings.Join(whereClauses, " AND ")
	}

	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM waste_pickups %s", whereSQL)
	var total int64
	if err := r.pool.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("failed to count pickups: %w", err)
	}

	offset := (filter.Page - 1) * filter.Limit
	selectQuery := fmt.Sprintf(`
		SELECT id, household_id, type, status, pickup_date, safety_check, created_at, updated_at
		FROM waste_pickups
		%s
		ORDER BY created_at DESC, id DESC
		LIMIT $%d OFFSET $%d
	`, whereSQL, argIdx, argIdx+1)

	argsWithPaging := append(args, filter.Limit, offset)

	rows, err := r.pool.Query(ctx, selectQuery, argsWithPaging...)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to query pickups: %w", err)
	}
	defer rows.Close()

	pickups := make([]domain.Pickup, 0)
	for rows.Next() {
		var p domain.Pickup
		if err := rows.Scan(&p.ID, &p.HouseholdID, &p.Type, &p.Status, &p.PickupDate, &p.SafetyCheck, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, 0, fmt.Errorf("failed to scan pickup: %w", err)
		}
		pickups = append(pickups, p)
	}

	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("rows iteration error: %w", err)
	}

	return pickups, total, nil
}

// UpdateSchedule schedules a pickup atomically.
func (r *PickupRepository) UpdateSchedule(ctx context.Context, id string, pickupDate time.Time, safetyCheck *bool, updatedAt time.Time) (*domain.Pickup, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return nil, fmt.Errorf("failed to begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	// Lock row
	queryLock := `
		SELECT id, household_id, type, status, pickup_date, safety_check, created_at, updated_at
		FROM waste_pickups
		WHERE id = $1
		FOR UPDATE
	`
	var p domain.Pickup
	err = tx.QueryRow(ctx, queryLock, id).Scan(
		&p.ID,
		&p.HouseholdID,
		&p.Type,
		&p.Status,
		&p.PickupDate,
		&p.SafetyCheck,
		&p.CreatedAt,
		&p.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, fmt.Errorf("failed to query pickup: %w", err)
	}

	// State validation
	if p.Status != domain.PickupStatusPending {
		return nil, domain.ErrInvalidStateTransition
	}

	// Effective safety check
	effectiveSafety := p.SafetyCheck
	if p.Type == domain.WasteTypeElectronic {
		if safetyCheck != nil {
			effectiveSafety = safetyCheck
		}
		if effectiveSafety == nil || !*effectiveSafety {
			return nil, domain.ErrSafetyCheckRequired
		}
	} else {
		effectiveSafety = nil
	}

	updateQuery := `
		UPDATE waste_pickups
		SET status = $1, pickup_date = $2, safety_check = $3, updated_at = $4
		WHERE id = $5
		RETURNING id, household_id, type, status, pickup_date, safety_check, created_at, updated_at
	`
	var updated domain.Pickup
	err = tx.QueryRow(ctx, updateQuery, domain.PickupStatusScheduled, pickupDate, effectiveSafety, updatedAt, id).Scan(
		&updated.ID,
		&updated.HouseholdID,
		&updated.Type,
		&updated.Status,
		&updated.PickupDate,
		&updated.SafetyCheck,
		&updated.CreatedAt,
		&updated.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to update scheduled pickup: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("failed to commit schedule tx: %w", err)
	}

	return &updated, nil
}

// UpdateCancel cancels a pending or scheduled pickup atomically.
func (r *PickupRepository) UpdateCancel(ctx context.Context, id string, updatedAt time.Time) (*domain.Pickup, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return nil, fmt.Errorf("failed to begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	queryLock := `
		SELECT id, household_id, type, status, pickup_date, safety_check, created_at, updated_at
		FROM waste_pickups
		WHERE id = $1
		FOR UPDATE
	`
	var p domain.Pickup
	err = tx.QueryRow(ctx, queryLock, id).Scan(
		&p.ID,
		&p.HouseholdID,
		&p.Type,
		&p.Status,
		&p.PickupDate,
		&p.SafetyCheck,
		&p.CreatedAt,
		&p.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, fmt.Errorf("failed to query pickup: %w", err)
	}

	if p.Status != domain.PickupStatusPending && p.Status != domain.PickupStatusScheduled {
		return nil, domain.ErrInvalidStateTransition
	}

	// If canceling from scheduled, retains pickup_date for history. If canceling from pending, pickup_date remains null.
	updateQuery := `
		UPDATE waste_pickups
		SET status = $1, updated_at = $2
		WHERE id = $3
		RETURNING id, household_id, type, status, pickup_date, safety_check, created_at, updated_at
	`
	var canceled domain.Pickup
	err = tx.QueryRow(ctx, updateQuery, domain.PickupStatusCanceled, updatedAt, id).Scan(
		&canceled.ID,
		&canceled.HouseholdID,
		&canceled.Type,
		&canceled.Status,
		&canceled.PickupDate,
		&canceled.SafetyCheck,
		&canceled.CreatedAt,
		&canceled.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to cancel pickup: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("failed to commit cancel tx: %w", err)
	}

	return &canceled, nil
}
