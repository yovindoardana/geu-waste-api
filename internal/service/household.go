package service

import (
	"context"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/yovindoardana/geu-waste-api/internal/domain"
	"github.com/yovindoardana/geu-waste-api/internal/repository/postgres"
)

// ValidationError represents validation failures for specific fields.
type ValidationError struct {
	Field   string
	Message string
}

func (v *ValidationError) Error() string {
	return fmt.Sprintf("%s: %s", v.Field, v.Message)
}

// HouseholdService handles business logic for households.
type HouseholdService struct {
	repo *postgres.HouseholdRepository
}

// NewHouseholdService creates a new HouseholdService.
func NewHouseholdService(repo *postgres.HouseholdRepository) *HouseholdService {
	return &HouseholdService{repo: repo}
}

// CreateHousehold validates input and persists a new household.
func (s *HouseholdService) CreateHousehold(ctx context.Context, req domain.CreateHouseholdRequest) (*domain.Household, error) {
	if req.OwnerName == nil {
		return nil, &ValidationError{Field: "owner_name", Message: "owner_name is required"}
	}
	ownerName := strings.TrimSpace(*req.OwnerName)
	if ownerName == "" {
		return nil, &ValidationError{Field: "owner_name", Message: "owner_name cannot be empty"}
	}
	if utf8.RuneCountInString(ownerName) > 150 {
		return nil, &ValidationError{Field: "owner_name", Message: "owner_name cannot exceed 150 characters"}
	}

	if req.Address == nil {
		return nil, &ValidationError{Field: "address", Message: "address is required"}
	}
	address := strings.TrimSpace(*req.Address)
	if address == "" {
		return nil, &ValidationError{Field: "address", Message: "address cannot be empty"}
	}
	if utf8.RuneCountInString(address) > 1000 {
		return nil, &ValidationError{Field: "address", Message: "address cannot exceed 1000 characters"}
	}

	now := time.Now().UTC()
	h := &domain.Household{
		ID:        uuid.New().String(),
		OwnerName: ownerName,
		Address:   address,
		CreatedAt: now,
		UpdatedAt: now,
	}

	if err := s.repo.Create(ctx, h); err != nil {
		return nil, err
	}

	return h, nil
}

// GetHousehold retrieves a household by its ID.
func (s *HouseholdService) GetHousehold(ctx context.Context, id string) (*domain.Household, error) {
	return s.repo.FindByID(ctx, id)
}

// ListHouseholds retrieves paginated households.
func (s *HouseholdService) ListHouseholds(ctx context.Context, page, limit int) ([]domain.Household, int64, error) {
	return s.repo.FindAll(ctx, page, limit)
}

// DeleteHousehold deletes a household without dependencies.
func (s *HouseholdService) DeleteHousehold(ctx context.Context, id string) error {
	return s.repo.Delete(ctx, id)
}
