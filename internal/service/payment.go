package service

import (
	"context"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"github.com/yovindoardana/geu-waste-api/internal/domain"
	"github.com/yovindoardana/geu-waste-api/internal/repository/postgres"
	"github.com/yovindoardana/geu-waste-api/internal/validator"
)

var amountRegex = regexp.MustCompile(`^[0-9]+(\.[0-9]{1,2})?$`)

// PaymentService handles payment and invoice business logic.
type PaymentService struct {
	repo       *postgres.PaymentRepository
	pickupRepo *postgres.PickupRepository
}

// NewPaymentService creates a new PaymentService.
func NewPaymentService(repo *postgres.PaymentRepository, pickupRepo *postgres.PickupRepository) *PaymentService {
	return &PaymentService{
		repo:       repo,
		pickupRepo: pickupRepo,
	}
}

// CompletePickup completes a pickup and creates its payment invoice atomically (BR04, D01).
func (s *PaymentService) CompletePickup(ctx context.Context, pickupID string) (*domain.PickupCompleteResult, error) {
	pickup, err := s.pickupRepo.FindByID(ctx, pickupID)
	if err != nil {
		return nil, err
	}

	if pickup.Status != domain.PickupStatusScheduled {
		return nil, domain.ErrInvalidStateTransition
	}

	tariff := domain.GetTariffForType(pickup.Type)
	paymentID := uuid.New().String()
	now := time.Now().UTC()

	completedPickup, payment, err := s.repo.CompletePickupAndCreatePayment(ctx, pickupID, paymentID, tariff, now)
	if err != nil {
		return nil, err
	}

	return &domain.PickupCompleteResult{
		Pickup:  completedPickup,
		Payment: payment,
	}, nil
}

// EnsurePayment handles manual POST /api/payments invoice creation/retrieval (D01).
func (s *PaymentService) EnsurePayment(ctx context.Context, req domain.CreatePaymentRequest) (*domain.Payment, bool, error) {
	if req.HouseholdID == nil {
		return nil, false, &ValidationError{Field: "household_id", Message: "household_id is required"}
	}
	householdID, err := validator.ParseUUID(*req.HouseholdID)
	if err != nil {
		return nil, false, &ValidationError{Field: "household_id", Message: err.Error()}
	}

	if req.WasteID == nil {
		return nil, false, &ValidationError{Field: "waste_id", Message: "waste_id is required"}
	}
	wasteID, err := validator.ParseUUID(*req.WasteID)
	if err != nil {
		return nil, false, &ValidationError{Field: "waste_id", Message: err.Error()}
	}

	if req.Amount == nil {
		return nil, false, &ValidationError{Field: "amount", Message: "amount is required"}
	}
	amountStr := strings.TrimSpace(*req.Amount)
	if !amountRegex.MatchString(amountStr) {
		return nil, false, &ValidationError{Field: "amount", Message: "amount must be a positive decimal string with up to 2 decimal places (e.g. '50000.00')"}
	}

	parsedAmount, err := decimal.NewFromString(amountStr)
	if err != nil || parsedAmount.LessThanOrEqual(decimal.Zero) || parsedAmount.GreaterThan(decimal.NewFromFloat(9999999999.99)) {
		return nil, false, &ValidationError{Field: "amount", Message: "amount must be positive and not exceed 9999999999.99"}
	}

	paymentID := uuid.New().String()
	now := time.Now().UTC()

	payment, isCreated, err := s.repo.EnsurePaymentForPickup(ctx, paymentID, householdID, wasteID, parsedAmount, now)
	if err != nil {
		if strings.Contains(err.Error(), "mismatch") {
			return nil, false, &ValidationError{Field: "amount", Message: err.Error()}
		}
		return nil, false, err
	}

	return payment, isCreated, nil
}

// GetPayment retrieves a payment by ID.
func (s *PaymentService) GetPayment(ctx context.Context, id string) (*domain.Payment, error) {
	return s.repo.FindByID(ctx, id)
}

// ListPayments retrieves filtered payments.
func (s *PaymentService) ListPayments(ctx context.Context, filter domain.PaymentFilter) ([]domain.Payment, int64, error) {
	return s.repo.FindAll(ctx, filter)
}
