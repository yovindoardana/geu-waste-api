package seed

import (
	"context"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"
	"github.com/yovindoardana/geu-waste-api/internal/config"
)

// Fixture constants as defined in docs/database.md Section 9
const (
	// Households
	HouseholdAID = "10000000-0000-4000-8000-000000000001"
	HouseholdBID = "10000000-0000-4000-8000-000000000002"
	HouseholdCID = "10000000-0000-4000-8000-000000000003"
	HouseholdDID = "10000000-0000-4000-8000-000000000004"

	// Pickups
	PickupW1ID = "20000000-0000-4000-8000-000000000001"
	PickupW2ID = "20000000-0000-4000-8000-000000000002"
	PickupW3ID = "20000000-0000-4000-8000-000000000003"
	PickupW4ID = "20000000-0000-4000-8000-000000000004"
	PickupW5ID = "20000000-0000-4000-8000-000000000005"
	PickupW6ID = "20000000-0000-4000-8000-000000000006"
	PickupW7ID = "20000000-0000-4000-8000-000000000007"

	// Payments
	PaymentP1ID = "30000000-0000-4000-8000-000000000001"
	PaymentP2ID = "30000000-0000-4000-8000-000000000002"
	PaymentP3ID = "30000000-0000-4000-8000-000000000003"

	// Seed Sample Proof
	SeedProofFilename = "40000000-0000-4000-8000-000000000001.png"
	SeedProofURL      = "/uploads/payment-proofs/40000000-0000-4000-8000-000000000001.png"
)

// HouseholdFixture represents a seed household record.
type HouseholdFixture struct {
	ID        string
	OwnerName string
	Address   string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// PickupFixture represents a seed waste pickup record.
type PickupFixture struct {
	ID          string
	HouseholdID string
	Type        string
	Status      string
	PickupDate  *time.Time
	SafetyCheck *bool
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// PaymentFixture represents a seed payment record.
type PaymentFixture struct {
	ID           string
	HouseholdID  string
	WasteID      string
	Amount       decimal.Decimal
	PaymentDate  *time.Time
	Status       string
	ProofFileURL *string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// GetFixtures returns the exact deterministic fixtures according to docs/database.md.
func GetFixtures() ([]HouseholdFixture, []PickupFixture, []PaymentFixture) {
	tBase, _ := time.Parse(time.RFC3339, "2026-10-01T00:00:00Z")
	tW2, _ := time.Parse(time.RFC3339, "2026-10-02T02:00:00Z")
	tW4, _ := time.Parse(time.RFC3339, "2026-10-08T02:00:00Z")
	tW5, _ := time.Parse(time.RFC3339, "2026-10-03T02:00:00Z")
	tW6, _ := time.Parse(time.RFC3339, "2026-10-04T02:00:00Z")
	tW7, _ := time.Parse(time.RFC3339, "2026-10-01T01:00:00Z")
	tP2Paid, _ := time.Parse(time.RFC3339, "2026-10-05T03:00:00Z")

	boolFalse := false
	boolTrue := true
	proofURL := SeedProofURL

	households := []HouseholdFixture{
		{ID: HouseholdAID, OwnerName: "Seed Household A", Address: "Jalan Contoh A No. 1", CreatedAt: tBase, UpdatedAt: tBase},
		{ID: HouseholdBID, OwnerName: "Seed Household B", Address: "Jalan Contoh B No. 2", CreatedAt: tBase, UpdatedAt: tBase},
		{ID: HouseholdCID, OwnerName: "Seed Household C", Address: "Jalan Contoh C No. 3", CreatedAt: tBase, UpdatedAt: tBase},
		{ID: HouseholdDID, OwnerName: "Seed Household D", Address: "Jalan Contoh D No. 4", CreatedAt: tBase, UpdatedAt: tBase},
	}

	pickups := []PickupFixture{
		{ID: PickupW1ID, HouseholdID: HouseholdAID, Type: "organic", Status: "pending", PickupDate: nil, SafetyCheck: nil, CreatedAt: tBase, UpdatedAt: tBase},
		{ID: PickupW2ID, HouseholdID: HouseholdAID, Type: "plastic", Status: "completed", PickupDate: &tW2, SafetyCheck: nil, CreatedAt: tBase, UpdatedAt: tW2},
		{ID: PickupW3ID, HouseholdID: HouseholdBID, Type: "electronic", Status: "pending", PickupDate: nil, SafetyCheck: &boolFalse, CreatedAt: tBase, UpdatedAt: tBase},
		{ID: PickupW4ID, HouseholdID: HouseholdCID, Type: "paper", Status: "scheduled", PickupDate: &tW4, SafetyCheck: nil, CreatedAt: tBase, UpdatedAt: tW4},
		{ID: PickupW5ID, HouseholdID: HouseholdCID, Type: "electronic", Status: "completed", PickupDate: &tW5, SafetyCheck: &boolTrue, CreatedAt: tBase, UpdatedAt: tW5},
		{ID: PickupW6ID, HouseholdID: HouseholdDID, Type: "paper", Status: "completed", PickupDate: &tW6, SafetyCheck: nil, CreatedAt: tBase, UpdatedAt: tW6},
		{ID: PickupW7ID, HouseholdID: HouseholdDID, Type: "organic", Status: "canceled", PickupDate: nil, SafetyCheck: nil, CreatedAt: tBase, UpdatedAt: tW7},
	}

	payments := []PaymentFixture{
		{
			ID:           PaymentP1ID,
			HouseholdID:  HouseholdAID,
			WasteID:      PickupW2ID,
			Amount:       decimal.NewFromInt(50000),
			PaymentDate:  nil,
			Status:       "pending",
			ProofFileURL: nil,
			CreatedAt:    tW2,
			UpdatedAt:    tW2,
		},
		{
			ID:           PaymentP2ID,
			HouseholdID:  HouseholdCID,
			WasteID:      PickupW5ID,
			Amount:       decimal.NewFromInt(100000),
			PaymentDate:  &tP2Paid,
			Status:       "paid",
			ProofFileURL: &proofURL,
			CreatedAt:    tW5,
			UpdatedAt:    tP2Paid,
		},
		{
			ID:           PaymentP3ID,
			HouseholdID:  HouseholdDID,
			WasteID:      PickupW6ID,
			Amount:       decimal.NewFromInt(50000),
			PaymentDate:  nil,
			Status:       "failed",
			ProofFileURL: nil,
			CreatedAt:    tW6,
			UpdatedAt:    tW6,
		},
	}

	return households, pickups, payments
}

// EnsureSampleProofAssets ensures valid PNG proof assets exist locally and in the upload destination.
func EnsureSampleProofAssets(uploadDir string) error {
	// Find project root if running from submodule/test directory
	baseDir := "."
	if _, err := os.Stat("go.mod"); os.IsNotExist(err) {
		if _, err := os.Stat("../../go.mod"); err == nil {
			baseDir = "../.."
		}
	}

	seedDir := filepath.Join(baseDir, "seeds", "assets")
	postmanDir := filepath.Join(baseDir, "postman", "assets")

	if err := os.MkdirAll(seedDir, 0755); err != nil {
		return fmt.Errorf("failed to create seeds/assets dir: %w", err)
	}
	if err := os.MkdirAll(postmanDir, 0755); err != nil {
		return fmt.Errorf("failed to create postman/assets dir: %w", err)
	}
	if err := os.MkdirAll(uploadDir, 0755); err != nil {
		return fmt.Errorf("failed to create upload directory %q: %w", uploadDir, err)
	}

	seedAssetPath := filepath.Join(seedDir, "sample-proof.png")
	postmanAssetPath := filepath.Join(postmanDir, "sample-proof.png")
	targetUploadPath := filepath.Join(uploadDir, SeedProofFilename)

	// Create sample image if seedAssetPath doesn't exist
	if _, err := os.Stat(seedAssetPath); os.IsNotExist(err) {
		img := image.NewRGBA(image.Rect(0, 0, 200, 200))
		for x := 0; x < 200; x++ {
			for y := 0; y < 200; y++ {
				img.Set(x, y, color.RGBA{R: 70, G: 130, B: 180, A: 255})
			}
		}
		f, err := os.Create(seedAssetPath)
		if err != nil {
			return fmt.Errorf("failed to create seed asset %q: %w", seedAssetPath, err)
		}
		if err := png.Encode(f, img); err != nil {
			f.Close()
			return fmt.Errorf("failed to encode seed asset: %w", err)
		}
		f.Close()
	}

	// Copy to postman assets if not exists
	if _, err := os.Stat(postmanAssetPath); os.IsNotExist(err) {
		_ = copyFile(seedAssetPath, postmanAssetPath)
	}

	// Copy to target upload path (for seeded paid payment P2)
	if _, err := os.Stat(targetUploadPath); os.IsNotExist(err) {
		if err := copyFile(seedAssetPath, targetUploadPath); err != nil {
			return fmt.Errorf("failed to copy seed proof to upload target %q: %w", targetUploadPath, err)
		}
	}

	return nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}

// Run executes the idempotent database seed inside a single database transaction.
func Run(ctx context.Context, pool *pgxpool.Pool, cfg *config.Config) error {
	log.Println("Starting deterministic seed execution...")

	// 1. Ensure sample assets
	if err := EnsureSampleProofAssets(cfg.UploadDir); err != nil {
		return fmt.Errorf("failed to prepare sample proof assets: %w", err)
	}

	// 2. Begin transaction
	tx, err := pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	households, pickups, payments := GetFixtures()

	// 3. Seed Households
	for _, h := range households {
		query := `
			INSERT INTO households (id, owner_name, address, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5)
			ON CONFLICT (id) DO NOTHING;
		`
		if _, err := tx.Exec(ctx, query, h.ID, h.OwnerName, h.Address, h.CreatedAt, h.UpdatedAt); err != nil {
			return fmt.Errorf("failed to seed household %s: %w", h.ID, err)
		}
	}
	log.Printf("Seeded %d households.", len(households))

	// 4. Seed Pickups
	for _, p := range pickups {
		query := `
			INSERT INTO waste_pickups (id, household_id, type, status, pickup_date, safety_check, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
			ON CONFLICT (id) DO NOTHING;
		`
		if _, err := tx.Exec(ctx, query, p.ID, p.HouseholdID, p.Type, p.Status, p.PickupDate, p.SafetyCheck, p.CreatedAt, p.UpdatedAt); err != nil {
			return fmt.Errorf("failed to seed pickup %s: %w", p.ID, err)
		}
	}
	log.Printf("Seeded %d pickups.", len(pickups))

	// 5. Seed Payments
	for _, py := range payments {
		query := `
			INSERT INTO payments (id, household_id, waste_id, amount, payment_date, status, proof_file_url, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
			ON CONFLICT (id) DO NOTHING;
		`
		if _, err := tx.Exec(ctx, query, py.ID, py.HouseholdID, py.WasteID, py.Amount, py.PaymentDate, py.Status, py.ProofFileURL, py.CreatedAt, py.UpdatedAt); err != nil {
			return fmt.Errorf("failed to seed payment %s: %w", py.ID, err)
		}
	}
	log.Printf("Seeded %d payments.", len(payments))

	// 6. Commit transaction
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit seed transaction: %w", err)
	}

	log.Println("Database seed completed successfully.")
	return nil
}
