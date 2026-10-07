package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"
	"github.com/yovindoardana/geu-waste-api/internal/domain"
)

// ReportRepository implements aggregation queries for reports.
type ReportRepository struct {
	pool *pgxpool.Pool
}

// NewReportRepository creates a new ReportRepository.
func NewReportRepository(pool *pgxpool.Pool) *ReportRepository {
	return &ReportRepository{pool: pool}
}

// GetWasteSummary calculates 16 canonical buckets for waste pickup status.
func (r *ReportRepository) GetWasteSummary(ctx context.Context) (*domain.WasteSummaryReport, error) {
	query := `
		SELECT type, status, COUNT(*)
		FROM waste_pickups
		GROUP BY type, status
	`
	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query waste summary: %w", err)
	}
	defer rows.Close()

	counts := make(map[string]int64)
	for rows.Next() {
		var wType, status string
		var count int64
		if err := rows.Scan(&wType, &status, &count); err != nil {
			return nil, fmt.Errorf("failed to scan waste summary row: %w", err)
		}
		counts[wType+":"+status] = count
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("waste summary rows iteration error: %w", err)
	}

	items := make([]domain.WasteSummaryItem, 0, 16)
	var totalPickups int64

	for _, wType := range domain.DefaultWasteTypesOrder {
		for _, status := range domain.DefaultPickupStatusesOrder {
			cnt := counts[wType+":"+status]
			items = append(items, domain.WasteSummaryItem{
				Type:   wType,
				Status: status,
				Count:  cnt,
			})
			totalPickups += cnt
		}
	}

	return &domain.WasteSummaryReport{
		Items:        items,
		TotalPickups: totalPickups,
	}, nil
}

// GetPaymentSummary calculates the 3 status buckets and revenue.
func (r *ReportRepository) GetPaymentSummary(ctx context.Context) (*domain.PaymentSummaryReport, error) {
	query := `
		SELECT status, COUNT(*), COALESCE(SUM(amount), 0)
		FROM payments
		GROUP BY status
	`
	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query payment summary: %w", err)
	}
	defer rows.Close()

	counts := make(map[string]int64)
	sums := make(map[string]decimal.Decimal)

	for rows.Next() {
		var status string
		var count int64
		var sum decimal.Decimal
		if err := rows.Scan(&status, &count, &sum); err != nil {
			return nil, fmt.Errorf("failed to scan payment summary row: %w", err)
		}
		counts[status] = count
		sums[status] = sum
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("payment summary rows iteration error: %w", err)
	}

	items := make([]domain.PaymentSummaryItem, 0, 3)
	var totalPayments int64
	var totalRevenue decimal.Decimal

	for _, status := range domain.DefaultPaymentStatusesOrder {
		cnt := counts[status]
		sum := sums[status]
		items = append(items, domain.PaymentSummaryItem{
			Status:      status,
			Count:       cnt,
			TotalAmount: domain.ValueFromDecimal(sum),
		})
		totalPayments += cnt
		if status == domain.PaymentStatusPaid {
			totalRevenue = sum
		}
	}

	return &domain.PaymentSummaryReport{
		Currency:      "IDR",
		Items:         items,
		TotalPayments: totalPayments,
		TotalRevenue:  domain.ValueFromDecimal(totalRevenue),
	}, nil
}
