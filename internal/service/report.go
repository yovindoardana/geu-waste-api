package service

import (
	"context"
	"fmt"

	"github.com/yovindoardana/geu-waste-api/internal/domain"
	"github.com/yovindoardana/geu-waste-api/internal/repository/postgres"
)

// ReportService handles report aggregation retrieval.
type ReportService struct {
	repo *postgres.ReportRepository
}

// NewReportService creates a new ReportService.
func NewReportService(repo *postgres.ReportRepository) *ReportService {
	return &ReportService{repo: repo}
}

// GetWasteSummary retrieves the 16-bucket waste pickup summary.
func (s *ReportService) GetWasteSummary(ctx context.Context) (*domain.WasteSummaryReport, error) {
	if s.repo == nil {
		return nil, fmt.Errorf("report repository not available")
	}
	return s.repo.GetWasteSummary(ctx)
}

// GetPaymentSummary retrieves the payment statistics and total revenue.
func (s *ReportService) GetPaymentSummary(ctx context.Context) (*domain.PaymentSummaryReport, error) {
	if s.repo == nil {
		return nil, fmt.Errorf("report repository not available")
	}
	return s.repo.GetPaymentSummary(ctx)
}
