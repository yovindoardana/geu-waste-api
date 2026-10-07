package postgres

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"
	"github.com/yovindoardana/geu-waste-api/internal/domain"
)

// PaymentRepository implements database operations for payments and atomic billing workflows.
type PaymentRepository struct {
	pool *pgxpool.Pool
}

// NewPaymentRepository creates a new PaymentRepository.
func NewPaymentRepository(pool *pgxpool.Pool) *PaymentRepository {
	return &PaymentRepository{pool: pool}
}

// FindByID retrieves a payment by primary key UUID.
func (r *PaymentRepository) FindByID(ctx context.Context, id string) (*domain.Payment, error) {
	query := `
		SELECT id, household_id, waste_id, amount, payment_date, status, proof_file_url, created_at, updated_at
		FROM payments
		WHERE id = $1
	`
	var p domain.Payment
	var rawAmount decimal.Decimal
	err := r.pool.QueryRow(ctx, query, id).Scan(
		&p.ID,
		&p.HouseholdID,
		&p.WasteID,
		&rawAmount,
		&p.PaymentDate,
		&p.Status,
		&p.ProofFileURL,
		&p.CreatedAt,
		&p.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, fmt.Errorf("failed to query payment: %w", err)
	}
	p.Amount = domain.ValueFromDecimal(rawAmount)
	return &p, nil
}

// FindByWasteID retrieves a payment associated with a waste pickup.
func (r *PaymentRepository) FindByWasteID(ctx context.Context, wasteID string) (*domain.Payment, error) {
	query := `
		SELECT id, household_id, waste_id, amount, payment_date, status, proof_file_url, created_at, updated_at
		FROM payments
		WHERE waste_id = $1
	`
	var p domain.Payment
	var rawAmount decimal.Decimal
	err := r.pool.QueryRow(ctx, query, wasteID).Scan(
		&p.ID,
		&p.HouseholdID,
		&p.WasteID,
		&rawAmount,
		&p.PaymentDate,
		&p.Status,
		&p.ProofFileURL,
		&p.CreatedAt,
		&p.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, fmt.Errorf("failed to query payment by waste id: %w", err)
	}
	p.Amount = domain.ValueFromDecimal(rawAmount)
	return &p, nil
}

// CompletePickupAndCreatePayment completes a scheduled pickup and creates an invoice payment atomically.
func (r *PaymentRepository) CompletePickupAndCreatePayment(
	ctx context.Context,
	pickupID string,
	paymentID string,
	tariff decimal.Decimal,
	now time.Time,
) (*domain.Pickup, *domain.Payment, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return nil, nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	// 1. Lock pickup row
	queryLock := `
		SELECT id, household_id, type, status, pickup_date, safety_check, created_at, updated_at
		FROM waste_pickups
		WHERE id = $1
		FOR UPDATE
	`
	var p domain.Pickup
	err = tx.QueryRow(ctx, queryLock, pickupID).Scan(
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
			return nil, nil, domain.ErrNotFound
		}
		return nil, nil, fmt.Errorf("failed to lock pickup: %w", err)
	}

	// Must be in scheduled state to complete
	if p.Status != domain.PickupStatusScheduled {
		return nil, nil, domain.ErrInvalidStateTransition
	}

	// 2. Update pickup to completed
	updatePickupQuery := `
		UPDATE waste_pickups
		SET status = $1, updated_at = $2
		WHERE id = $3
		RETURNING id, household_id, type, status, pickup_date, safety_check, created_at, updated_at
	`
	var completedPickup domain.Pickup
	err = tx.QueryRow(ctx, updatePickupQuery, domain.PickupStatusCompleted, now, pickupID).Scan(
		&completedPickup.ID,
		&completedPickup.HouseholdID,
		&completedPickup.Type,
		&completedPickup.Status,
		&completedPickup.PickupDate,
		&completedPickup.SafetyCheck,
		&completedPickup.CreatedAt,
		&completedPickup.UpdatedAt,
	)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to complete pickup: %w", err)
	}

	// 3. Create payment invoice
	insertPaymentQuery := `
		INSERT INTO payments (id, household_id, waste_id, amount, payment_date, status, proof_file_url, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id, household_id, waste_id, amount, payment_date, status, proof_file_url, created_at, updated_at
	`
	var payment domain.Payment
	var rawAmount decimal.Decimal
	err = tx.QueryRow(
		ctx,
		insertPaymentQuery,
		paymentID,
		completedPickup.HouseholdID,
		completedPickup.ID,
		tariff,
		nil,
		domain.PaymentStatusPending,
		nil,
		now,
		now,
	).Scan(
		&payment.ID,
		&payment.HouseholdID,
		&payment.WasteID,
		&rawAmount,
		&payment.PaymentDate,
		&payment.Status,
		&payment.ProofFileURL,
		&payment.CreatedAt,
		&payment.UpdatedAt,
	)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create payment invoice: %w", err)
	}
	payment.Amount = domain.ValueFromDecimal(rawAmount)

	if err := tx.Commit(ctx); err != nil {
		return nil, nil, fmt.Errorf("failed to commit completion transaction: %w", err)
	}

	return &completedPickup, &payment, nil
}

// EnsurePaymentForPickup guarantees one invoice exists for a completed pickup (POST /api/payments, D01).
func (r *PaymentRepository) EnsurePaymentForPickup(
	ctx context.Context,
	paymentID string,
	householdID string,
	wasteID string,
	amount decimal.Decimal,
	now time.Time,
) (*domain.Payment, bool, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return nil, false, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	// 1. Lock and lookup pickup
	queryPickup := `
		SELECT id, household_id, type, status
		FROM waste_pickups
		WHERE id = $1
		FOR UPDATE
	`
	var p domain.Pickup
	err = tx.QueryRow(ctx, queryPickup, wasteID).Scan(&p.ID, &p.HouseholdID, &p.Type, &p.Status)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, false, domain.ErrNotFound
		}
		return nil, false, fmt.Errorf("failed to query pickup: %w", err)
	}

	// Ownership check
	if p.HouseholdID != householdID {
		return nil, false, errors.New("ownership mismatch: pickup does not belong to specified household")
	}

	// Must be completed
	if p.Status != domain.PickupStatusCompleted {
		return nil, false, domain.ErrInvalidStateTransition
	}

	// Tariff check
	expectedTariff := domain.GetTariffForType(p.Type)
	if !amount.Equal(expectedTariff) {
		return nil, false, fmt.Errorf("amount mismatch: expected %s, got %s", expectedTariff.StringFixed(2), amount.StringFixed(2))
	}

	// 2. Check if payment already exists
	queryExisting := `
		SELECT id, household_id, waste_id, amount, payment_date, status, proof_file_url, created_at, updated_at
		FROM payments
		WHERE waste_id = $1
		FOR UPDATE
	`
	var existing domain.Payment
	var rawAmount decimal.Decimal
	err = tx.QueryRow(ctx, queryExisting, wasteID).Scan(
		&existing.ID,
		&existing.HouseholdID,
		&existing.WasteID,
		&rawAmount,
		&existing.PaymentDate,
		&existing.Status,
		&existing.ProofFileURL,
		&existing.CreatedAt,
		&existing.UpdatedAt,
	)
	if err == nil {
		// Existing invoice found
		existing.Amount = domain.ValueFromDecimal(rawAmount)
		if existing.HouseholdID != householdID || !existing.Amount.Equal(expectedTariff) {
			return nil, false, domain.ErrPaymentConflict
		}
		if err := tx.Commit(ctx); err != nil {
			return nil, false, fmt.Errorf("failed to commit: %w", err)
		}
		return &existing, false, nil // 200 OK (already exists)
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return nil, false, fmt.Errorf("failed to query existing payment: %w", err)
	}

	// 3. Insert new payment (201 Created)
	insertQuery := `
		INSERT INTO payments (id, household_id, waste_id, amount, payment_date, status, proof_file_url, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id, household_id, waste_id, amount, payment_date, status, proof_file_url, created_at, updated_at
	`
	var newPayment domain.Payment
	err = tx.QueryRow(
		ctx,
		insertQuery,
		paymentID,
		householdID,
		wasteID,
		amount,
		nil,
		domain.PaymentStatusPending,
		nil,
		now,
		now,
	).Scan(
		&newPayment.ID,
		&newPayment.HouseholdID,
		&newPayment.WasteID,
		&rawAmount,
		&newPayment.PaymentDate,
		&newPayment.Status,
		&newPayment.ProofFileURL,
		&newPayment.CreatedAt,
		&newPayment.UpdatedAt,
	)
	if err != nil {
		return nil, false, fmt.Errorf("failed to insert payment: %w", err)
	}
	newPayment.Amount = domain.ValueFromDecimal(rawAmount)

	if err := tx.Commit(ctx); err != nil {
		return nil, false, fmt.Errorf("failed to commit transaction: %w", err)
	}

	return &newPayment, true, nil // 201 Created
}

// FindAll retrieves paginated payments with filters.
func (r *PaymentRepository) FindAll(ctx context.Context, filter domain.PaymentFilter) ([]domain.Payment, int64, error) {
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

	if filter.StartDate != nil {
		whereClauses = append(whereClauses, fmt.Sprintf("payment_date >= $%d", argIdx))
		args = append(args, *filter.StartDate)
		argIdx++
	}

	if filter.EndDate != nil {
		whereClauses = append(whereClauses, fmt.Sprintf("payment_date < $%d", argIdx))
		args = append(args, *filter.EndDate)
		argIdx++
	}

	whereSQL := ""
	if len(whereClauses) > 0 {
		whereSQL = "WHERE " + strings.Join(whereClauses, " AND ")
	}

	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM payments %s", whereSQL)
	var total int64
	if err := r.pool.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("failed to count payments: %w", err)
	}

	offset := (filter.Page - 1) * filter.Limit
	selectQuery := fmt.Sprintf(`
		SELECT id, household_id, waste_id, amount, payment_date, status, proof_file_url, created_at, updated_at
		FROM payments
		%s
		ORDER BY created_at DESC, id DESC
		LIMIT $%d OFFSET $%d
	`, whereSQL, argIdx, argIdx+1)

	argsWithPaging := append(args, filter.Limit, offset)

	rows, err := r.pool.Query(ctx, selectQuery, argsWithPaging...)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to query payments: %w", err)
	}
	defer rows.Close()

	payments := make([]domain.Payment, 0)
	for rows.Next() {
		var p domain.Payment
		var rawAmount decimal.Decimal
		if err := rows.Scan(&p.ID, &p.HouseholdID, &p.WasteID, &rawAmount, &p.PaymentDate, &p.Status, &p.ProofFileURL, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, 0, fmt.Errorf("failed to scan payment row: %w", err)
		}
		p.Amount = domain.ValueFromDecimal(rawAmount)
		payments = append(payments, p)
	}

	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("rows iteration error: %w", err)
	}

	return payments, total, nil
}

// ConfirmPayment confirms a pending payment with proof image URL and payment date atomically.
func (r *PaymentRepository) ConfirmPayment(
	ctx context.Context,
	paymentID string,
	proofFileURL string,
	paymentDate time.Time,
) (*domain.Payment, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	// Lock payment row
	queryLock := `
		SELECT id, household_id, waste_id, amount, payment_date, status, proof_file_url, created_at, updated_at
		FROM payments
		WHERE id = $1
		FOR UPDATE
	`
	var p domain.Payment
	var rawAmount decimal.Decimal
	err = tx.QueryRow(ctx, queryLock, paymentID).Scan(
		&p.ID,
		&p.HouseholdID,
		&p.WasteID,
		&rawAmount,
		&p.PaymentDate,
		&p.Status,
		&p.ProofFileURL,
		&p.CreatedAt,
		&p.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, fmt.Errorf("failed to query payment for update: %w", err)
	}

	if p.Status != domain.PaymentStatusPending {
		return nil, domain.ErrInvalidStateTransition
	}

	updateQuery := `
		UPDATE payments
		SET status = $1, payment_date = $2, proof_file_url = $3, updated_at = $2
		WHERE id = $4
		RETURNING id, household_id, waste_id, amount, payment_date, status, proof_file_url, created_at, updated_at
	`
	var confirmed domain.Payment
	err = tx.QueryRow(
		ctx,
		updateQuery,
		domain.PaymentStatusPaid,
		paymentDate,
		proofFileURL,
		paymentID,
	).Scan(
		&confirmed.ID,
		&confirmed.HouseholdID,
		&confirmed.WasteID,
		&rawAmount,
		&confirmed.PaymentDate,
		&confirmed.Status,
		&confirmed.ProofFileURL,
		&confirmed.CreatedAt,
		&confirmed.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to update payment to paid: %w", err)
	}
	confirmed.Amount = domain.ValueFromDecimal(rawAmount)

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("failed to commit confirm transaction: %w", err)
	}

	return &confirmed, nil
}

