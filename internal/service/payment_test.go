package service

import (
	"bytes"
	"context"
	"errors"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/yovindoardana/geu-waste-api/internal/domain"
)

var sampleValidPNG = []byte{
	0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a,
	0x00, 0x00, 0x00, 0x0d, 0x49, 0x48, 0x44, 0x52,
	0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
	0x08, 0x06, 0x00, 0x00, 0x00, 0x1f, 0x15, 0xc4,
	0x89, 0x00, 0x00, 0x00, 0x0a, 0x49, 0x44, 0x41,
	0x54, 0x78, 0x9c, 0x63, 0x00, 0x01, 0x00, 0x00,
	0x05, 0x00, 0x01, 0x0d, 0x0a, 0x2d, 0xb4, 0x00,
	0x00, 0x00, 0x00, 0x49, 0x45, 0x4e, 0x44, 0xae,
	0x42, 0x60, 0x82,
}

type mockPaymentRepo struct {
	findPayment        *domain.Payment
	findErr            error
	confirmPayment     *domain.Payment
	confirmErr         error
	confirmCalled      bool
	confirmProofURL    string
	confirmPaymentID   string
	confirmPaymentDate time.Time
}

func (m *mockPaymentRepo) FindByID(ctx context.Context, id string) (*domain.Payment, error) {
	if m.findErr != nil {
		return nil, m.findErr
	}
	if m.findPayment != nil {
		return m.findPayment, nil
	}
	return nil, domain.ErrNotFound
}

func (m *mockPaymentRepo) ConfirmPayment(ctx context.Context, paymentID string, proofFileURL string, paymentDate time.Time) (*domain.Payment, error) {
	m.confirmCalled = true
	m.confirmPaymentID = paymentID
	m.confirmProofURL = proofFileURL
	m.confirmPaymentDate = paymentDate
	if m.confirmErr != nil {
		return nil, m.confirmErr
	}
	if m.confirmPayment != nil {
		return m.confirmPayment, nil
	}
	return &domain.Payment{
		ID:           paymentID,
		Status:       domain.PaymentStatusPaid,
		ProofFileURL: &proofFileURL,
		PaymentDate:  &paymentDate,
	}, nil
}

func (m *mockPaymentRepo) CompletePickupAndCreatePayment(ctx context.Context, pickupID string, paymentID string, tariff decimal.Decimal, now time.Time) (*domain.Pickup, *domain.Payment, error) {
	return nil, nil, nil
}

func (m *mockPaymentRepo) EnsurePaymentForPickup(ctx context.Context, paymentID string, householdID string, wasteID string, amount decimal.Decimal, now time.Time) (*domain.Payment, bool, error) {
	return nil, false, nil
}

func (m *mockPaymentRepo) FindAll(ctx context.Context, filter domain.PaymentFilter) ([]domain.Payment, int64, error) {
	return nil, 0, nil
}

func createTestMultipartHeader(t *testing.T, filename string, content []byte) *multipart.FileHeader {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("proof", filename)
	if err != nil {
		t.Fatalf("failed to create form file: %v", err)
	}
	if _, err := part.Write(content); err != nil {
		t.Fatalf("failed to write part content: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("failed to close writer: %v", err)
	}

	req, err := http.NewRequest("POST", "/test", &body)
	if err != nil {
		t.Fatalf("failed to create test request: %v", err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	if err := req.ParseMultipartForm(10 << 20); err != nil {
		t.Fatalf("failed to parse multipart form: %v", err)
	}

	files := req.MultipartForm.File["proof"]
	if len(files) == 0 {
		t.Fatalf("no files in parsed multipart form")
	}
	return files[0]
}

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

func TestConfirmPaymentWithProof_StorageAndDatabaseMatrix(t *testing.T) {
	ctx := context.Background()
	paymentID := "30000000-0000-4000-8000-000000000001"

	t.Run("Success: DB updated to paid and proof file exists on disk", func(t *testing.T) {
		tempDir := t.TempDir()
		mockRepo := &mockPaymentRepo{
			findPayment: &domain.Payment{
				ID:     paymentID,
				Status: domain.PaymentStatusPending,
			},
		}
		svc := NewPaymentService(mockRepo, nil)
		header := createTestMultipartHeader(t, "proof.png", sampleValidPNG)

		confirmed, err := svc.ConfirmPaymentWithProof(ctx, paymentID, header, tempDir)
		if err != nil {
			t.Fatalf("expected success, got error: %v", err)
		}
		if confirmed.Status != domain.PaymentStatusPaid {
			t.Errorf("expected status paid, got %s", confirmed.Status)
		}
		if confirmed.ProofFileURL == nil || *confirmed.ProofFileURL == "" {
			t.Fatalf("expected non-empty ProofFileURL")
		}

		// Verify file really exists on disk in uploadDir
		filename := filepath.Base(*confirmed.ProofFileURL)
		finalDiskPath := filepath.Join(tempDir, filename)
		fileInfo, err := os.Stat(finalDiskPath)
		if err != nil {
			t.Fatalf("expected file to exist at %s: %v", finalDiskPath, err)
		}
		if fileInfo.Size() != int64(len(sampleValidPNG)) {
			t.Errorf("expected file size %d, got %d", len(sampleValidPNG), fileInfo.Size())
		}
		if !mockRepo.confirmCalled {
			t.Errorf("expected ConfirmPayment to be called on repo")
		}
	})

	t.Run("DB error before commit: final file is cleaned up and payment remains pending", func(t *testing.T) {
		tempDir := t.TempDir()
		dbErr := errors.New("simulated database foreign key constraint error")
		mockRepo := &mockPaymentRepo{
			findPayment: &domain.Payment{
				ID:     paymentID,
				Status: domain.PaymentStatusPending,
			},
			confirmErr: dbErr,
		}
		svc := NewPaymentService(mockRepo, nil)
		header := createTestMultipartHeader(t, "proof.png", sampleValidPNG)

		_, err := svc.ConfirmPaymentWithProof(ctx, paymentID, header, tempDir)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !errors.Is(err, dbErr) {
			t.Errorf("expected dbErr, got %v", err)
		}

		// Ensure uploadDir has NO files left (both staging and final cleaned up)
		entries, err := os.ReadDir(tempDir)
		if err != nil {
			t.Fatalf("failed to read temp dir: %v", err)
		}
		if len(entries) != 0 {
			t.Errorf("expected 0 files in temp dir after DB error cleanup, found %d", len(entries))
		}
	})

	t.Run("Commit uncertain error: final file is PRESERVED on disk for reconciliation", func(t *testing.T) {
		tempDir := t.TempDir()
		mockRepo := &mockPaymentRepo{
			findPayment: &domain.Payment{
				ID:     paymentID,
				Status: domain.PaymentStatusPending,
			},
			confirmErr: domain.ErrCommitUncertain,
		}
		svc := NewPaymentService(mockRepo, nil)
		header := createTestMultipartHeader(t, "proof.png", sampleValidPNG)

		_, err := svc.ConfirmPaymentWithProof(ctx, paymentID, header, tempDir)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !errors.Is(err, domain.ErrCommitUncertain) {
			t.Errorf("expected ErrCommitUncertain, got %v", err)
		}

		// Ensure final file was PRESERVED
		entries, err := os.ReadDir(tempDir)
		if err != nil {
			t.Fatalf("failed to read temp dir: %v", err)
		}
		if len(entries) != 1 {
			t.Fatalf("expected exactly 1 preserved file in temp dir, found %d", len(entries))
		}
		if entries[0].Name()[:8] == "staging_" {
			t.Errorf("preserved file should be final file, not staging file: %s", entries[0].Name())
		}
	})

	t.Run("Payment not pending: rejected before file I/O, DB not called", func(t *testing.T) {
		tempDir := t.TempDir()
		mockRepo := &mockPaymentRepo{
			findPayment: &domain.Payment{
				ID:     paymentID,
				Status: domain.PaymentStatusPaid, // Already paid
			},
		}
		svc := NewPaymentService(mockRepo, nil)
		header := createTestMultipartHeader(t, "proof.png", sampleValidPNG)

		_, err := svc.ConfirmPaymentWithProof(ctx, paymentID, header, tempDir)
		if err != domain.ErrInvalidStateTransition {
			t.Fatalf("expected ErrInvalidStateTransition, got %v", err)
		}
		if mockRepo.confirmCalled {
			t.Errorf("ConfirmPayment should not have been called")
		}
		entries, _ := os.ReadDir(tempDir)
		if len(entries) != 0 {
			t.Errorf("expected 0 files created, found %d", len(entries))
		}
	})

	t.Run("Invalid file content: rejected by validator before DB interaction", func(t *testing.T) {
		tempDir := t.TempDir()
		mockRepo := &mockPaymentRepo{
			findPayment: &domain.Payment{
				ID:     paymentID,
				Status: domain.PaymentStatusPending,
			},
		}
		svc := NewPaymentService(mockRepo, nil)
		invalidHeader := createTestMultipartHeader(t, "fake.png", []byte("not a valid image format"))

		_, err := svc.ConfirmPaymentWithProof(ctx, paymentID, invalidHeader, tempDir)
		if err == nil {
			t.Fatal("expected validation error, got nil")
		}
		if mockRepo.confirmCalled {
			t.Errorf("ConfirmPayment should not have been called on validation failure")
		}
		entries, _ := os.ReadDir(tempDir)
		if len(entries) != 0 {
			t.Errorf("expected 0 files created, found %d", len(entries))
		}
	})
}
