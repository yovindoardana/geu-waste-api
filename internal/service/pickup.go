package service

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/yovindoardana/geu-waste-api/internal/domain"
	"github.com/yovindoardana/geu-waste-api/internal/repository/postgres"
	"github.com/yovindoardana/geu-waste-api/internal/validator"
)

// PickupService handles business operations and state machines for waste pickups.
type PickupService struct {
	repo *postgres.PickupRepository
}

// NewPickupService creates a new PickupService.
func NewPickupService(repo *postgres.PickupRepository) *PickupService {
	return &PickupService{repo: repo}
}

// CreatePickup validates input and creates a new waste pickup.
func (s *PickupService) CreatePickup(ctx context.Context, req domain.CreatePickupRequest) (*domain.Pickup, error) {
	if req.HouseholdID == nil {
		return nil, &ValidationError{Field: "household_id", Message: "household_id is required"}
	}
	householdID, err := validator.ParseUUID(*req.HouseholdID)
	if err != nil {
		return nil, &ValidationError{Field: "household_id", Message: err.Error()}
	}

	if req.Type == nil {
		return nil, &ValidationError{Field: "type", Message: "type is required"}
	}
	wasteType := strings.TrimSpace(*req.Type)
	if !domain.ValidWasteTypes[wasteType] {
		return nil, &ValidationError{Field: "type", Message: "invalid waste type: must be organic, plastic, paper, or electronic"}
	}

	// Electronic requires safety_check boolean upon creation (false is valid)
	if wasteType == domain.WasteTypeElectronic {
		if req.SafetyCheck == nil {
			return nil, &ValidationError{Field: "safety_check", Message: "safety_check is required for electronic waste"}
		}
	} else {
		// Non-electronic must NOT have safety_check provided
		if req.SafetyCheck != nil {
			return nil, &ValidationError{Field: "safety_check", Message: "safety_check must not be provided for non-electronic waste"}
		}
	}

	now := time.Now().UTC()
	pickup := &domain.Pickup{
		ID:          uuid.New().String(),
		HouseholdID: householdID,
		Type:        wasteType,
		Status:      domain.PickupStatusPending,
		PickupDate:  nil,
		SafetyCheck: req.SafetyCheck,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	if err := s.repo.CreateWithHouseholdLock(ctx, pickup); err != nil {
		return nil, err
	}

	return pickup, nil
}

// SchedulePickup schedules a pending pickup.
func (s *PickupService) SchedulePickup(ctx context.Context, id string, req domain.SchedulePickupRequest) (*domain.Pickup, error) {
	if req.PickupDate == nil {
		return nil, &ValidationError{Field: "pickup_date", Message: "pickup_date is required"}
	}

	dateStr := strings.TrimSpace(*req.PickupDate)
	if dateStr == "" {
		return nil, &ValidationError{Field: "pickup_date", Message: "pickup_date cannot be empty"}
	}

	parsedDate, err := time.Parse(time.RFC3339, dateStr)
	if err != nil {
		return nil, &ValidationError{Field: "pickup_date", Message: "pickup_date must be a valid RFC3339 timestamp with offset (e.g. 2026-10-08T09:00:00+07:00)"}
	}
	normalizedDate := parsedDate.UTC()

	// Get current pickup to check type restrictions
	existing, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if existing.Type != domain.WasteTypeElectronic && req.SafetyCheck != nil {
		return nil, &ValidationError{Field: "safety_check", Message: "safety_check must not be provided for non-electronic waste"}
	}

	now := time.Now().UTC()
	return s.repo.UpdateSchedule(ctx, id, normalizedDate, req.SafetyCheck, now)
}

// CancelPickup cancels a pending or scheduled pickup.
func (s *PickupService) CancelPickup(ctx context.Context, id string) (*domain.Pickup, error) {
	now := time.Now().UTC()
	return s.repo.UpdateCancel(ctx, id, now)
}

// GetPickup retrieves a pickup by ID.
func (s *PickupService) GetPickup(ctx context.Context, id string) (*domain.Pickup, error) {
	return s.repo.FindByID(ctx, id)
}

// ListPickups lists paginated pickups matching filters.
func (s *PickupService) ListPickups(ctx context.Context, filter domain.PickupFilter) ([]domain.Pickup, int64, error) {
	if filter.Status != nil && *filter.Status != "" {
		if !domain.ValidPickupStatuses[*filter.Status] {
			return nil, 0, &ValidationError{Field: "status", Message: "invalid status filter"}
		}
	}
	if filter.HouseholdID != nil && *filter.HouseholdID != "" {
		parsed, err := validator.ParseUUID(*filter.HouseholdID)
		if err != nil {
			return nil, 0, &ValidationError{Field: "household_id", Message: "invalid household_id filter"}
		}
		filter.HouseholdID = &parsed
	}
	return s.repo.FindAll(ctx, filter)
}
