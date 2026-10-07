package service

import (
	"context"
	"testing"

	"github.com/yovindoardana/geu-waste-api/internal/domain"
)

func TestPickupValidation(t *testing.T) {
	svc := NewPickupService(nil)

	t.Run("Missing household_id", func(t *testing.T) {
		wasteType := "organic"
		_, err := svc.CreatePickup(context.Background(), domain.CreatePickupRequest{
			HouseholdID: nil,
			Type:        &wasteType,
		})
		if err == nil {
			t.Fatal("expected error for missing household_id, got nil")
		}
	})

	t.Run("Invalid waste type", func(t *testing.T) {
		hid := "10000000-0000-4000-8000-000000000001"
		invalidType := "toxic"
		_, err := svc.CreatePickup(context.Background(), domain.CreatePickupRequest{
			HouseholdID: &hid,
			Type:        &invalidType,
		})
		if err == nil {
			t.Fatal("expected error for invalid type, got nil")
		}
	})

	t.Run("Electronic waste requires safety_check", func(t *testing.T) {
		hid := "10000000-0000-4000-8000-000000000001"
		wasteType := "electronic"
		_, err := svc.CreatePickup(context.Background(), domain.CreatePickupRequest{
			HouseholdID: &hid,
			Type:        &wasteType,
			SafetyCheck: nil,
		})
		if err == nil {
			t.Fatal("expected error for electronic waste without safety_check, got nil")
		}
	})

	t.Run("Non-electronic waste rejects safety_check", func(t *testing.T) {
		hid := "10000000-0000-4000-8000-000000000001"
		wasteType := "organic"
		safety := true
		_, err := svc.CreatePickup(context.Background(), domain.CreatePickupRequest{
			HouseholdID: &hid,
			Type:        &wasteType,
			SafetyCheck: &safety,
		})
		if err == nil {
			t.Fatal("expected error for organic waste with safety_check, got nil")
		}
	})
}
