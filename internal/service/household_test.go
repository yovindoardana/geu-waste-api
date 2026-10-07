package service

import (
	"context"
	"testing"

	"github.com/yovindoardana/geu-waste-api/internal/domain"
)

func TestHouseholdValidation(t *testing.T) {
	svc := NewHouseholdService(nil) // Testing input validation without DB

	t.Run("Missing owner_name", func(t *testing.T) {
		addr := "Jalan Test"
		_, err := svc.CreateHousehold(context.Background(), domain.CreateHouseholdRequest{
			OwnerName: nil,
			Address:   &addr,
		})
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("Empty owner_name", func(t *testing.T) {
		name := "   "
		addr := "Jalan Test"
		_, err := svc.CreateHousehold(context.Background(), domain.CreateHouseholdRequest{
			OwnerName: &name,
			Address:   &addr,
		})
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("Missing address", func(t *testing.T) {
		name := "Owner Test"
		_, err := svc.CreateHousehold(context.Background(), domain.CreateHouseholdRequest{
			OwnerName: &name,
			Address:   nil,
		})
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("Empty address", func(t *testing.T) {
		name := "Owner Test"
		addr := "   "
		_, err := svc.CreateHousehold(context.Background(), domain.CreateHouseholdRequest{
			OwnerName: &name,
			Address:   &addr,
		})
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})
}
