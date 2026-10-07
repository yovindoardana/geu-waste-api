package integration

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/golang-migrate/migrate/v4"
	migratepostgres "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/lib/pq"
	"github.com/shopspring/decimal"
	"github.com/yovindoardana/geu-waste-api/internal/config"
	"github.com/yovindoardana/geu-waste-api/internal/domain"
	"github.com/yovindoardana/geu-waste-api/internal/repository/postgres"
	"github.com/yovindoardana/geu-waste-api/internal/service"
)

var (
	migrateOnce sync.Once
	migrateErr  error
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

func checkTestDatabaseSafety(testDBName, appDBName string) error {
	if testDBName == appDBName || testDBName == "geu_waste" {
		return fmt.Errorf("FATAL SAFETY CHECK: Integration tests are forbidden from executing against application database %q. Target must be an isolated test database (default: geu_waste_test)", testDBName)
	}
	return nil
}

func setupTestDB(t *testing.T) (*pgxpool.Pool, func()) {
	t.Helper()

	testDBName := os.Getenv("TEST_DB_NAME")
	if testDBName == "" {
		testDBName = "geu_waste_test"
	}

	appDBName := os.Getenv("DB_NAME")
	if appDBName == "" {
		appDBName = "geu_waste"
	}

	// SAFETY RULE: Strictly refuse to execute tests against application or production DB
	if err := checkTestDatabaseSafety(testDBName, appDBName); err != nil {
		t.Fatalf("%v", err)
	}

	cfg, err := config.Load()
	if err != nil {
		cfg = &config.Config{
			AppEnv:           "test",
			AppPort:          8080,
			DBHost:           "localhost",
			DBPort:           5432,
			DBName:           testDBName,
			DBUser:           "postgres",
			DBPassword:       "postgres",
			DBSSLMode:        "disable",
			UploadDir:        t.TempDir(),
			DBConnectTimeout: 5 * time.Second,
			ShutdownTimeout:  10 * time.Second,
		}
	} else {
		// Override DBName to isolated test database
		cfg.DBName = testDBName
	}

	// 1. Ensure test database exists by querying admin DB (postgres)
	adminDSN := fmt.Sprintf("postgres://%s:%s@%s:%d/postgres?sslmode=%s",
		cfg.DBUser, cfg.DBPassword, cfg.DBHost, cfg.DBPort, cfg.DBSSLMode)

	adminDB, err := sql.Open("postgres", adminDSN)
	if err != nil {
		t.Fatalf("Failed to connect to PostgreSQL admin database: %v", err)
	}
	defer adminDB.Close()

	var exists bool
	err = adminDB.QueryRow(`SELECT EXISTS(SELECT 1 FROM pg_database WHERE datname = $1)`, testDBName).Scan(&exists)
	if err != nil {
		t.Fatalf("Failed to check if test database exists: %v", err)
	}
	if !exists {
		_, err = adminDB.Exec(fmt.Sprintf(`CREATE DATABASE "%s"`, testDBName))
		if err != nil {
			t.Fatalf("Failed to create test database %q: %v", testDBName, err)
		}
	}

	// 2. Apply migrations to test database once per test binary execution
	migrateOnce.Do(func() {
		testSQLDSN := cfg.DSN()
		testSQLDB, err := sql.Open("postgres", testSQLDSN)
		if err != nil {
			migrateErr = fmt.Errorf("failed to open sql connection for test migration: %w", err)
			return
		}
		defer testSQLDB.Close()

		driver, err := migratepostgres.WithInstance(testSQLDB, &migratepostgres.Config{
			MigrationsTable: "schema_migrations",
		})
		if err != nil {
			migrateErr = fmt.Errorf("failed to create migration driver: %w", err)
			return
		}

		migrationsDir := os.Getenv("MIGRATIONS_DIR")
		if migrationsDir == "" {
			if _, err := os.Stat("migrations"); err == nil {
				migrationsDir = "migrations"
			} else if _, err := os.Stat("../../migrations"); err == nil {
				migrationsDir = "../../migrations"
			} else {
				migrationsDir = "migrations"
			}
		}
		absPath, err := filepath.Abs(migrationsDir)
		if err != nil {
			migrateErr = fmt.Errorf("failed to resolve migrations dir %q: %w", migrationsDir, err)
			return
		}

		m, err := migrate.NewWithDatabaseInstance("file://"+absPath, testDBName, driver)
		if err != nil {
			migrateErr = fmt.Errorf("failed to initialize migrate engine: %w", err)
			return
		}

		if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
			migrateErr = fmt.Errorf("failed to apply migrations up to test db: %w", err)
			return
		}
	})

	if migrateErr != nil {
		t.Fatalf("Test migration setup failed: %v", migrateErr)
	}

	// 3. Connect pgx connection pool to test database
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	pool, err := postgres.NewPool(ctx, cfg)
	if err != nil {
		t.Fatalf("Failed to connect pgx pool to test database %q: %v", testDBName, err)
	}

	// 4. Truncate tables to ensure isolated and repeatable test state
	truncateCtx, truncateCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer truncateCancel()
	_, err = pool.Exec(truncateCtx, `TRUNCATE TABLE payments, waste_pickups, households CASCADE`)
	if err != nil {
		t.Fatalf("Failed to truncate tables in test database: %v", err)
	}

	cleanup := func() {
		cleanCtx, cleanCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanCancel()
		_, _ = pool.Exec(cleanCtx, `TRUNCATE TABLE payments, waste_pickups, households CASCADE`)
		pool.Close()
	}

	return pool, cleanup
}

func createTestMultipartFileHeader(t *testing.T, filename string, content []byte) *multipart.FileHeader {
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

func TestBR01_ControlledSequence_CompletionFirstThenCreateBlocked(t *testing.T) {
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
		OwnerName: "Lock Order User 1",
		Address:   "Jl. Lock 1",
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := householdRepo.Create(ctx, household); err != nil {
		t.Fatalf("failed to create household: %v", err)
	}

	typeStr := domain.WasteTypeOrganic
	p1, err := pickupSvc.CreatePickup(ctx, domain.CreatePickupRequest{
		HouseholdID: &household.ID,
		Type:        &typeStr,
	})
	if err != nil {
		t.Fatalf("failed to create p1: %v", err)
	}
	scheduledDate := now.Add(time.Hour)
	if _, err := pickupRepo.UpdateSchedule(ctx, p1.ID, scheduledDate, nil, now); err != nil {
		t.Fatalf("failed to schedule p1: %v", err)
	}

	// 1. Completion obtains lock first and commits
	completeRes, err := paymentSvc.CompletePickup(ctx, p1.ID)
	if err != nil {
		t.Fatalf("expected completion to succeed, got %v", err)
	}
	if completeRes.Payment.Status != domain.PaymentStatusPending {
		t.Fatalf("expected pending payment, got %s", completeRes.Payment.Status)
	}

	// 2. Next create pickup on same household MUST be rejected by BR01
	_, err = pickupSvc.CreatePickup(ctx, domain.CreatePickupRequest{
		HouseholdID: &household.ID,
		Type:        &typeStr,
	})
	if err != domain.ErrHouseholdPendingPayment {
		t.Fatalf("expected ErrHouseholdPendingPayment (BR01), got %v", err)
	}

	// 3. Verify exact DB counts
	var pickupCount, paymentCount int
	_ = pool.QueryRow(ctx, "SELECT COUNT(*) FROM waste_pickups WHERE household_id = $1", household.ID).Scan(&pickupCount)
	_ = pool.QueryRow(ctx, "SELECT COUNT(*) FROM payments WHERE household_id = $1", household.ID).Scan(&paymentCount)

	if pickupCount != 1 {
		t.Errorf("expected exactly 1 pickup in DB, got %d", pickupCount)
	}
	if paymentCount != 1 {
		t.Errorf("expected exactly 1 payment in DB, got %d", paymentCount)
	}
}

func TestBR01_ControlledSequence_CreateFirstThenCompletionSucceeds(t *testing.T) {
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
		OwnerName: "Lock Order User 2",
		Address:   "Jl. Lock 2",
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := householdRepo.Create(ctx, household); err != nil {
		t.Fatalf("failed to create household: %v", err)
	}

	typeStr := domain.WasteTypeOrganic
	p1, err := pickupSvc.CreatePickup(ctx, domain.CreatePickupRequest{
		HouseholdID: &household.ID,
		Type:        &typeStr,
	})
	if err != nil {
		t.Fatalf("failed to create p1: %v", err)
	}
	scheduledDate := now.Add(time.Hour)
	if _, err := pickupRepo.UpdateSchedule(ctx, p1.ID, scheduledDate, nil, now); err != nil {
		t.Fatalf("failed to schedule p1: %v", err)
	}

	// 1. Create pickup p2 obtains lock first and commits (no pending payment yet)
	p2, err := pickupSvc.CreatePickup(ctx, domain.CreatePickupRequest{
		HouseholdID: &household.ID,
		Type:        &typeStr,
	})
	if err != nil {
		t.Fatalf("expected create p2 to succeed, got %v", err)
	}
	if p2.Status != domain.PickupStatusPending {
		t.Fatalf("expected p2 pending status, got %s", p2.Status)
	}

	// 2. Completion of p1 executes next
	completeRes, err := paymentSvc.CompletePickup(ctx, p1.ID)
	if err != nil {
		t.Fatalf("expected completion of p1 to succeed, got %v", err)
	}
	if completeRes.Payment.Status != domain.PaymentStatusPending {
		t.Fatalf("expected pending payment for p1, got %s", completeRes.Payment.Status)
	}

	// 3. Verify exact DB counts: 2 pickups, 1 payment
	var pickupCount, paymentCount int
	_ = pool.QueryRow(ctx, "SELECT COUNT(*) FROM waste_pickups WHERE household_id = $1", household.ID).Scan(&pickupCount)
	_ = pool.QueryRow(ctx, "SELECT COUNT(*) FROM payments WHERE household_id = $1", household.ID).Scan(&paymentCount)

	if pickupCount != 2 {
		t.Errorf("expected exactly 2 pickups in DB, got %d", pickupCount)
	}
	if paymentCount != 1 {
		t.Errorf("expected exactly 1 payment in DB, got %d", paymentCount)
	}
}

func TestPaymentConfirmationWithProof_StorageAndDatabaseHandling(t *testing.T) {
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
		OwnerName: "Proof Test User",
		Address:   "Jl. Proof No. 8",
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := householdRepo.Create(ctx, household); err != nil {
		t.Fatalf("failed to create household: %v", err)
	}

	typeStr := domain.WasteTypePlastic
	pickup, err := pickupSvc.CreatePickup(ctx, domain.CreatePickupRequest{
		HouseholdID: &household.ID,
		Type:        &typeStr,
	})
	if err != nil {
		t.Fatalf("failed to create pickup: %v", err)
	}

	scheduledDate := now.Add(time.Hour)
	if _, err := pickupRepo.UpdateSchedule(ctx, pickup.ID, scheduledDate, nil, now); err != nil {
		t.Fatalf("failed to schedule pickup: %v", err)
	}

	completeRes, err := paymentSvc.CompletePickup(ctx, pickup.ID)
	if err != nil {
		t.Fatalf("failed to complete pickup: %v", err)
	}

	tempDir := t.TempDir()

	// 1. Success case through PaymentService: writes file to staging, promotes to final, confirms DB, file exists on disk
	fileHeader := createTestMultipartFileHeader(t, "proof.png", sampleValidPNG)
	confirmedPayment, err := paymentSvc.ConfirmPaymentWithProof(ctx, completeRes.Payment.ID, fileHeader, tempDir)
	if err != nil {
		t.Fatalf("expected successful confirmation, got %v", err)
	}
	if confirmedPayment.Status != domain.PaymentStatusPaid {
		t.Errorf("expected paid status, got %s", confirmedPayment.Status)
	}
	if confirmedPayment.ProofFileURL == nil || *confirmedPayment.ProofFileURL == "" {
		t.Fatalf("expected non-empty ProofFileURL")
	}

	// Verify file really exists on disk
	finalDiskPath := filepath.Join(tempDir, filepath.Base(*confirmedPayment.ProofFileURL))
	if _, err := os.Stat(finalDiskPath); os.IsNotExist(err) {
		t.Errorf("expected proof file to exist on disk at %s", finalDiskPath)
	}

	// 2. Repeat confirmation must be rejected (409 INVALID_STATE_TRANSITION)
	repeatHeader := createTestMultipartFileHeader(t, "repeat.png", sampleValidPNG)
	_, err = paymentSvc.ConfirmPaymentWithProof(ctx, completeRes.Payment.ID, repeatHeader, tempDir)
	if err != domain.ErrInvalidStateTransition {
		t.Errorf("expected ErrInvalidStateTransition on second confirm, got %v", err)
	}
}

func TestSafetyCheck_RefuseAppDatabase(t *testing.T) {
	if err := checkTestDatabaseSafety("geu_waste", "geu_waste"); err == nil {
		t.Errorf("expected error when testDBName matches app database name, got nil")
	}
	if err := checkTestDatabaseSafety("geu_waste", "geu_waste_other"); err == nil {
		t.Errorf("expected error when testDBName is geu_waste, got nil")
	}
	if err := checkTestDatabaseSafety("geu_waste_test", "geu_waste"); err != nil {
		t.Errorf("expected valid test db name to pass safety check, got %v", err)
	}
}

