package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/yovindoardana/geu-waste-api/internal/config"
	"github.com/yovindoardana/geu-waste-api/internal/handler"
	"github.com/yovindoardana/geu-waste-api/internal/repository/postgres"
	"github.com/yovindoardana/geu-waste-api/internal/service"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Configuration error: %v", err)
	}

	if cfg.AppEnv == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Initialize database pool
	dbPool, err := postgres.NewPool(ctx, cfg)
	if err != nil {
		log.Fatalf("Failed to initialize database pool: %v", err)
	}
	defer dbPool.Close()
	log.Printf("Connected to database %s on %s:%d", cfg.DBName, cfg.DBHost, cfg.DBPort)

	// Setup Gin router
	router := setupRouter(dbPool)

	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.AppPort),
		Handler: router,
	}

	// Start HTTP server in a goroutine
	go func() {
		log.Printf("Starting GEU Waste API server on port %d (env: %s)", cfg.AppPort, cfg.AppEnv)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("HTTP server listen error: %v", err)
		}
	}()

	// Graceful shutdown on SIGINT or SIGTERM
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server gracefully...")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	log.Println("Server exited successfully.")
}

func setupRouter(dbPool *pgxpool.Pool) *gin.Engine {
	router := gin.New()
	router.Use(gin.Logger())
	router.Use(handler.RecoveryMiddleware())
	router.Use(handler.BodyLimitMiddleware(1 << 20)) // 1 MiB body limit for JSON

	router.HandleMethodNotAllowed = true
	router.NoRoute(handler.NoRouteHandler())
	router.NoMethod(handler.NoMethodHandler())

	// Health check
	healthHandler := handler.NewHealthHandler(dbPool)
	router.GET("/health", healthHandler.Health)

	// API routes group
	api := router.Group("/api")
	{
		// Household domain
		householdRepo := postgres.NewHouseholdRepository(dbPool)
		householdSvc := service.NewHouseholdService(householdRepo)
		householdHandler := handler.NewHouseholdHandler(householdSvc)

		api.POST("/households", householdHandler.Create)
		api.GET("/households", householdHandler.List)
		api.GET("/households/:id", householdHandler.GetByID)
		api.DELETE("/households/:id", householdHandler.Delete)
	}

	return router
}