package service

import (
	"context"
	"testing"

	"github.com/yovindoardana/geu-waste-api/internal/domain"
)

func TestPaymentValidation(t *testing.T) {
	svc := NewPaymentService(nil, nil)

	t.Run("Missing household_id", func(t *testing.T) {
		wasteID := "20000000-0000-4000-8000-000000000001"
		amount := "50000.00"
		_, _, err := svc.EnsurePayment(context.Background(), domain.CreatePaymentRequest{
			HouseholdID: nil,
			WasteID:     &wasteID,
			Amount:      &amount,
		})
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("Invalid amount format", func(t *testing.T) {
		hid := "10000000-0000-4000-8000-000000000001"
		wid := "20000000-0000-4000-8000-000000000001"
		invalidAmount := "50000.000" // 3 decimal places
		_, _, err := svc.EnsurePayment(context.Background(), domain.CreatePaymentRequest{
			HouseholdID: &hid,
			WasteID:     &wid,
			Amount:      &invalidAmount,
		})
		if err == nil {
			t.Fatal("expected error for 3 decimals, got nil")
		}
	})

	t.Run("Negative amount", func(t *testing.T) {
		hid := "10000000-0000-4000-8000-000000000001"
		wid := "20000000-0000-4000-8000-000000000001"
		negativeAmount := "-50000.00"
		_, _, err := svc.EnsurePayment(context.Background(), domain.CreatePaymentRequest{
			HouseholdID: &hid,
			WasteID:     &wid,
			Amount:      &negativeAmount,
		})
		if err == nil {
			t.Fatal("expected error for negative amount, got nil")
		}
	})
}
