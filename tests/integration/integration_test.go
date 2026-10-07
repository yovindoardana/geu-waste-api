package integration

import (
	"context"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"
	"github.com/yovindoardana/geu-waste-api/internal/config"
	"github.com/yovindoardana/geu-waste-api/internal/domain"
	"github.com/yovindoardana/geu-waste-api/internal/repository/postgres"
	"github.com/yovindoardana/geu-waste-api/internal/service"
)

func setupTestDB(t *testing.T) (*pgxpool.Pool, func()) {
	t.Helper()
	cfg, err := config.Load()
	if err != nil {
		// Fallback for direct integration run
		cfg = &config.Config{
			AppEnv:           "development",
			AppPort:          8080,
			DBHost:           "localhost",
			DBPort:           5432,
			DBName:           "geu_waste",
			DBUser:           "postgres",
			DBPassword:       "postgres",
			DBSSLMode:        "disable",
			UploadDir:        os.TempDir(),
			DBConnectTimeout: 5 * time.Second,
			ShutdownTimeout:  10 * time.Second,
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	pool, err := postgres.NewPool(ctx, cfg)
	if err != nil {
		t.Fatalf("Failed to connect to PostgreSQL test database: %v", err)
	}

	cleanup := func() {
		pool.Close()
	}

	return pool, cleanup
}

func TestAtomicCompletionAndBilling(t *testing.T) {
	pool, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()
	householdRepo := postgres.NewHouseholdRepository(pool)
	pickupRepo := postgres.NewPickupRepository(pool)
	paymentRepo := postgres.NewPaymentRepository(pool)
	paymentSvc := service.NewPaymentService(paymentRepo, pickupRepo)

	now := time.Now().UTC()
	household := &domain.Household{
		ID:        uuid.New().String(),
		OwnerName: "Integration Test User",
		Address:   "Jl. Integration No. 100",
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := householdRepo.Create(ctx, household); err != nil {
		t.Fatalf("failed to create test household: %v", err)
	}

	pickup := &domain.Pickup{
		ID:          uuid.New().String(),
		HouseholdID: household.ID,
		Type:        domain.WasteTypeOrganic,
		Status:      domain.PickupStatusPending,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := pickupRepo.CreateWithHouseholdLock(ctx, pickup); err != nil {
		t.Fatalf("failed to create test pickup: %v", err)
	}

	// Schedule pickup
	scheduledDate := now.Add(24 * time.Hour)
	scheduled, err := pickupRepo.UpdateSchedule(ctx, pickup.ID, scheduledDate, nil, now)
	if err != nil {
		t.Fatalf("failed to schedule pickup: %v", err)
	}
	if scheduled.Status != domain.PickupStatusScheduled {
		t.Fatalf("expected status scheduled, got %s", scheduled.Status)
	}

	// Complete pickup atomically
	result, err := paymentSvc.CompletePickup(ctx, pickup.ID)
	if err != nil {
		t.Fatalf("failed to complete pickup: %v", err)
	}

	if result.Pickup.Status != domain.PickupStatusCompleted {
		t.Errorf("expected completed pickup status, got %s", result.Pickup.Status)
	}
	if result.Payment.Status != domain.PaymentStatusPending {
		t.Errorf("expected pending payment status, got %s", result.Payment.Status)
	}
	if !result.Payment.Amount.Equal(decimal.NewFromInt(50000)) {
		t.Errorf("expected payment amount 50000.00, got %s", result.Payment.Amount.String())
	}
	if result.Payment.WasteID != pickup.ID {
		t.Errorf("expected waste_id %s, got %s", pickup.ID, result.Payment.WasteID)
	}

	// Assert only 1 payment row in database
	var paymentCount int
	err = pool.QueryRow(ctx, "SELECT COUNT(*) FROM payments WHERE waste_id = $1", pickup.ID).Scan(&paymentCount)
	if err != nil {
		t.Fatalf("failed to count payments: %v", err)
	}
	if paymentCount != 1 {
		t.Errorf("expected exactly 1 payment record, found %d", paymentCount)
	}
}

func TestConcurrency_CompletionDuplicateInvoicePrevention(t *testing.T) {
	pool, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()
	householdRepo := postgres.NewHouseholdRepository(pool)
	pickupRepo := postgres.NewPickupRepository(pool)
	paymentRepo := postgres.NewPaymentRepository(pool)
	paymentSvc := service.NewPaymentService(paymentRepo, pickupRepo)

	now := time.Now().UTC()
	household := &domain.Household{
		ID:        uuid.New().String(),
		OwnerName: "Concurrency Test User",
		Address:   "Jl. Concurrency No. 1",
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := householdRepo.Create(ctx, household); err != nil {
		t.Fatalf("failed to create test household: %v", err)
	}

	pickup := &domain.Pickup{
		ID:          uuid.New().String(),
		HouseholdID: household.ID,
		Type:        domain.WasteTypePlastic,
		Status:      domain.PickupStatusPending,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := pickupRepo.CreateWithHouseholdLock(ctx, pickup); err != nil {
		t.Fatalf("failed to create test pickup: %v", err)
	}

	scheduledDate := now.Add(24 * time.Hour)
	if _, err := pickupRepo.UpdateSchedule(ctx, pickup.ID, scheduledDate, nil, now); err != nil {
		t.Fatalf("failed to schedule pickup: %v", err)
	}

	// 10 concurrent completion requests
	concurrency := 10
	var wg sync.WaitGroup
	successCount := 0
	conflictCount := 0
	var mu sync.Mutex

	startBarrier := make(chan struct{})

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-startBarrier
			_, err := paymentSvc.CompletePickup(ctx, pickup.ID)
			mu.Lock()
			defer mu.Unlock()
			if err == nil {
				successCount++
			} else {
				conflictCount++
			}
		}()
	}

	close(startBarrier)
	wg.Wait()

	if successCount != 1 {
		t.Errorf("expected exactly 1 completion to succeed, got %d", successCount)
	}
	if conflictCount != concurrency-1 {
		t.Errorf("expected %d completions to be rejected, got %d", concurrency-1, conflictCount)
	}

	// Assert exactly 1 invoice in DB
	var paymentCount int
	err := pool.QueryRow(ctx, "SELECT COUNT(*) FROM payments WHERE waste_id = $1", pickup.ID).Scan(&paymentCount)
	if err != nil {
		t.Fatalf("failed to query payment count: %v", err)
	}
	if paymentCount != 1 {
		t.Errorf("expected exactly 1 payment record in DB, got %d", paymentCount)
	}
}

func TestBR01_HouseholdPendingPaymentLock(t *testing.T) {
	pool, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()
	householdRepo := postgres.NewHouseholdRepository(pool)
	pickupRepo := postgres.NewPickupRepository(pool)
	paymentRepo := postgres.NewPaymentRepository(pool)
	pickupSvc := service.NewPickupService(pickupRepo)
	paymentSvc := service.NewPaymentService(paymentRepo, pickupRepo)

	now := time.Now().UTC()
	household := &domain.Household{
		ID:        uuid.New().String(),
		OwnerName: "BR01 Test User",
		Address:   "Jl. BR01 No. 5",
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := householdRepo.Create(ctx, household); err != nil {
		t.Fatalf("failed to create household: %v", err)
	}

	// First pickup
	typeStr := domain.WasteTypePaper
	firstPickup, err := pickupSvc.CreatePickup(ctx, domain.CreatePickupRequest{
		HouseholdID: &household.ID,
		Type:        &typeStr,
	})
	if err != nil {
		t.Fatalf("failed to create first pickup: %v", err)
	}

	// Schedule and complete first pickup
	scheduledDate := now.Add(12 * time.Hour)
	if _, err := pickupRepo.UpdateSchedule(ctx, firstPickup.ID, scheduledDate, nil, now); err != nil {
		t.Fatalf("failed to schedule first pickup: %v", err)
	}
	completeRes, err := paymentSvc.CompletePickup(ctx, firstPickup.ID)
	if err != nil {
		t.Fatalf("failed to complete first pickup: %v", err)
	}

	// Household now has a pending payment
	// Attempt to create second pickup -> must be rejected (BR01)
	_, err = pickupSvc.CreatePickup(ctx, domain.CreatePickupRequest{
		HouseholdID: &household.ID,
		Type:        &typeStr,
	})
	if err != domain.ErrHouseholdPendingPayment {
		t.Errorf("expected ErrHouseholdPendingPayment, got %v", err)
	}

	// Confirm payment
	confirmDate := now.Add(2 * time.Hour)
	_, err = paymentRepo.ConfirmPayment(ctx, completeRes.Payment.ID, "/uploads/payment-proofs/test.png", confirmDate)
	if err != nil {
		t.Fatalf("failed to confirm payment: %v", err)
	}

	// After payment is paid, creating second pickup must succeed
	secondPickup, err := pickupSvc.CreatePickup(ctx, domain.CreatePickupRequest{
		HouseholdID: &household.ID,
		Type:        &typeStr,
	})
	if err != nil {
		t.Fatalf("expected successful pickup creation after payment paid, got %v", err)
	}
	if secondPickup.Status != domain.PickupStatusPending {
		t.Errorf("expected pending status, got %s", secondPickup.Status)
	}
}

func TestBR03_ElectronicSafetyCheck(t *testing.T) {
	pool, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()
	householdRepo := postgres.NewHouseholdRepository(pool)
	pickupRepo := postgres.NewPickupRepository(pool)
	pickupSvc := service.NewPickupService(pickupRepo)

	now := time.Now().UTC()
	household := &domain.Household{
		ID:        uuid.New().String(),
		OwnerName: "BR03 Test User",
		Address:   "Jl. Safety No. 9",
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := householdRepo.Create(ctx, household); err != nil {
		t.Fatalf("failed to create household: %v", err)
	}

	// 1. Electronic pickup with safety_check=false is allowed at creation
	falseVal := false
	typeStr := domain.WasteTypeElectronic
	electronicPickup, err := pickupSvc.CreatePickup(ctx, domain.CreatePickupRequest{
		HouseholdID: &household.ID,
		Type:        &typeStr,
		SafetyCheck: &falseVal,
	})
	if err != nil {
		t.Fatalf("expected create electronic pickup with safety_check=false to succeed, got %v", err)
	}

	// 2. Scheduling without updating safety_check=true must be rejected (BR03)
	pickupDateStr := now.Add(24 * time.Hour).Format(time.RFC3339)
	_, err = pickupSvc.SchedulePickup(ctx, electronicPickup.ID, domain.SchedulePickupRequest{
		PickupDate: &pickupDateStr,
	})
	if err != domain.ErrSafetyCheckRequired {
		t.Errorf("expected ErrSafetyCheckRequired, got %v", err)
	}

	// 3. Scheduling with safety_check=true must succeed
	trueVal := true
	scheduled, err := pickupSvc.SchedulePickup(ctx, electronicPickup.ID, domain.SchedulePickupRequest{
		PickupDate:  &pickupDateStr,
		SafetyCheck: &trueVal,
	})
	if err != nil {
		t.Fatalf("expected schedule to succeed with safety_check=true, got %v", err)
	}
	if scheduled.Status != domain.PickupStatusScheduled {
		t.Errorf("expected scheduled status, got %s", scheduled.Status)
	}
	if scheduled.SafetyCheck == nil || !*scheduled.SafetyCheck {
		t.Errorf("expected safety_check to be true")
	}
}

func TestReportsAggregation(t *testing.T) {
	pool, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()
	reportRepo := postgres.NewReportRepository(pool)

	wasteSummary, err := reportRepo.GetWasteSummary(ctx)
	if err != nil {
		t.Fatalf("failed to get waste summary: %v", err)
	}
	if len(wasteSummary.Items) != 16 {
		t.Fatalf("expected exactly 16 canonical buckets in waste summary, got %d", len(wasteSummary.Items))
	}

	// Verify canonical ordering
	expectedTypes := []string{"organic", "plastic", "paper", "electronic"}
	expectedStatuses := []string{"pending", "scheduled", "completed", "canceled"}
	idx := 0
	for _, expectedType := range expectedTypes {
		for _, expectedStatus := range expectedStatuses {
			item := wasteSummary.Items[idx]
			if item.Type != expectedType || item.Status != expectedStatus {
				t.Errorf("bucket %d expected %s/%s, got %s/%s", idx, expectedType, expectedStatus, item.Type, item.Status)
			}
			idx++
		}
	}

	paymentSummary, err := reportRepo.GetPaymentSummary(ctx)
	if err != nil {
		t.Fatalf("failed to get payment summary: %v", err)
	}
	if len(paymentSummary.Items) != 3 {
		t.Fatalf("expected 3 payment status items, got %d", len(paymentSummary.Items))
	}
	if paymentSummary.Currency != "IDR" {
		t.Errorf("expected currency IDR, got %s", paymentSummary.Currency)
	}
}
