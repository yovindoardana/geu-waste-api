package seed

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/shopspring/decimal"
)

func TestGetFixtures(t *testing.T) {
	households, pickups, payments := GetFixtures()

	// 1. Verify counts
	if len(households) != 4 {
		t.Errorf("expected 4 households, got %d", len(households))
	}
	if len(pickups) != 7 {
		t.Errorf("expected 7 pickups, got %d", len(pickups))
	}
	if len(payments) != 3 {
		t.Errorf("expected 3 payments, got %d", len(payments))
	}

	// 2. Verify payment status distribution
	pendingCount, paidCount, failedCount := 0, 0, 0
	totalRevenue := decimal.Zero
	for _, p := range payments {
		switch p.Status {
		case "pending":
			pendingCount++
		case "paid":
			paidCount++
			totalRevenue = totalRevenue.Add(p.Amount)
			if p.PaymentDate == nil {
				t.Errorf("paid payment %s must have payment_date", p.ID)
			}
			if p.ProofFileURL == nil || *p.ProofFileURL == "" {
				t.Errorf("paid payment %s must have proof_file_url", p.ID)
			}
		case "failed":
			failedCount++
		}
	}

	if pendingCount != 1 || paidCount != 1 || failedCount != 1 {
		t.Errorf("expected 1 pending, 1 paid, 1 failed, got %d pending, %d paid, %d failed", pendingCount, paidCount, failedCount)
	}

	expectedRevenue := decimal.NewFromInt(100000)
	if !totalRevenue.Equal(expectedRevenue) {
		t.Errorf("expected total revenue %s, got %s", expectedRevenue.StringFixed(2), totalRevenue.StringFixed(2))
	}

	// 3. Verify electronic safety check rules
	for _, p := range pickups {
		if p.Type == "electronic" {
			if p.SafetyCheck == nil {
				t.Errorf("electronic pickup %s must have non-nil safety_check", p.ID)
			}
			if p.Status == "completed" && !*p.SafetyCheck {
				t.Errorf("completed electronic pickup %s must have safety_check=true", p.ID)
			}
		} else {
			if p.SafetyCheck != nil {
				t.Errorf("non-electronic pickup %s must have nil safety_check", p.ID)
			}
		}
	}
}

func TestEnsureSampleProofAssets(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "geu_test_upload_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	if err := EnsureSampleProofAssets(tempDir); err != nil {
		t.Fatalf("EnsureSampleProofAssets() error = %v", err)
	}

	uploadFile := filepath.Join(tempDir, SeedProofFilename)
	if _, err := os.Stat(uploadFile); os.IsNotExist(err) {
		t.Errorf("expected upload target %s to exist", uploadFile)
	}
}
