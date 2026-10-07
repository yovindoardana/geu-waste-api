package main

import (
	"context"
	"log"

	"github.com/yovindoardana/geu-waste-api/internal/config"
	"github.com/yovindoardana/geu-waste-api/internal/repository/postgres"
	"github.com/yovindoardana/geu-waste-api/internal/seed"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Seed configuration error: %v", err)
	}

	ctx := context.Background()
	pool, err := postgres.NewPool(ctx, cfg)
	if err != nil {
		log.Fatalf("Database connection failed for seed: %v", err)
	}
	defer pool.Close()

	if err := seed.Run(ctx, pool, cfg); err != nil {
		log.Fatalf("Seed execution failed: %v", err)
	}

	log.Println("Seed process completed successfully.")
}
