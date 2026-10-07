package main

import (
	"database/sql"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	_ "github.com/lib/pq"
	"github.com/yovindoardana/geu-waste-api/internal/config"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Migration configuration error: %v", err)
	}

	db, err := sql.Open("postgres", cfg.DSN())
	if err != nil {
		log.Fatalf("Failed to connect to database for migration: %v", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		log.Fatalf("Failed to ping database for migration: %v", err)
	}

	driver, err := postgres.WithInstance(db, &postgres.Config{
		MigrationsTable: "schema_migrations",
	})
	if err != nil {
		log.Fatalf("Failed to create postgres migration driver: %v", err)
	}

	migrationsDir := os.Getenv("MIGRATIONS_DIR")
	if migrationsDir == "" {
		migrationsDir = "migrations"
	}
	absPath, err := filepath.Abs(migrationsDir)
	if err != nil {
		log.Fatalf("Failed to resolve migrations path: %v", err)
	}

	sourceURL := fmt.Sprintf("file://%s", absPath)
	m, err := migrate.NewWithDatabaseInstance(sourceURL, cfg.DBName, driver)
	if err != nil {
		log.Fatalf("Failed to initialize migration engine: %v", err)
	}

	cmd := "up"
	if len(os.Args) > 1 {
		cmd = os.Args[1]
	}

	switch cmd {
	case "up":
		log.Printf("Applying pending migrations from %s ...", sourceURL)
		if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
			log.Fatalf("Migration up failed: %v", err)
		}
		log.Println("Migrations applied successfully (up to date).")

	case "down":
		steps := 1
		if len(os.Args) > 2 {
			parsedSteps, err := strconv.Atoi(os.Args[2])
			if err != nil || parsedSteps <= 0 {
				log.Fatalf("Invalid steps parameter: %s", os.Args[2])
			}
			steps = parsedSteps
		}
		log.Printf("Rolling back %d migration step(s) ...", steps)
		if err := m.Steps(-steps); err != nil && !errors.Is(err, migrate.ErrNoChange) {
			log.Fatalf("Migration down failed: %v", err)
		}
		log.Printf("Rolled back %d migration step(s) successfully.", steps)

	case "version":
		version, dirty, err := m.Version()
		if err != nil {
			if errors.Is(err, migrate.ErrNilVersion) {
				log.Println("No migrations have been applied yet (version nil).")
				return
			}
			log.Fatalf("Failed to get migration version: %v", err)
		}
		log.Printf("Current migration version: %d (dirty: %v)", version, dirty)
		if dirty {
			log.Fatalf("Database migration is in dirty state!")
		}

	default:
		log.Fatalf("Unknown migration command: %s. Supported commands: up, down [steps], version", cmd)
	}
}
