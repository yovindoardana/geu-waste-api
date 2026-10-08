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
	router := setupRouter(dbPool, cfg.UploadDir)

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

func setupRouter(dbPool *pgxpool.Pool, uploadDir string) *gin.Engine {
	router := gin.New()
	router.Use(gin.Logger())
	router.Use(handler.RecoveryMiddleware())
	router.Use(handler.BodyLimitMiddleware(1<<20, 6<<20)) // 1 MiB for JSON, 6 MiB for multipart upload

	router.HandleMethodNotAllowed = true
	router.NoRoute(handler.NoRouteHandler())
	router.NoMethod(handler.NoMethodHandler())

	// Health check
	healthHandler := handler.NewHealthHandler(dbPool)
	router.GET("/health", healthHandler.Health)

	// Static uploaded proof serving
	uploadHandler := handler.NewUploadHandler(uploadDir)
	router.GET("/uploads/payment-proofs/:filename", uploadHandler.ServeFile)
	router.HEAD("/uploads/payment-proofs/:filename", uploadHandler.ServeFile)

	// API routes group
	api := router.Group("/api")
	{
		// Repositories
		householdRepo := postgres.NewHouseholdRepository(dbPool)
		pickupRepo := postgres.NewPickupRepository(dbPool)
		paymentRepo := postgres.NewPaymentRepository(dbPool)
		reportRepo := postgres.NewReportRepository(dbPool)

		// Services
		householdSvc := service.NewHouseholdService(householdRepo)
		pickupSvc := service.NewPickupService(pickupRepo)
		paymentSvc := service.NewPaymentService(paymentRepo, pickupRepo)
		reportSvc := service.NewReportService(reportRepo)

		// Handlers
		householdHandler := handler.NewHouseholdHandler(householdSvc)
		pickupHandler := handler.NewPickupHandler(pickupSvc, paymentSvc)
		paymentHandler := handler.NewPaymentHandler(paymentSvc, uploadDir)
		reportHandler := handler.NewReportHandler(reportSvc)

		// Household routes
		api.POST("/households", householdHandler.Create)
		api.GET("/households", householdHandler.List)
		api.GET("/households/:id", householdHandler.GetByID)
		api.DELETE("/households/:id", householdHandler.Delete)

		// Pickup routes
		api.POST("/pickups", pickupHandler.Create)
		api.GET("/pickups", pickupHandler.List)
		api.PUT("/pickups/:id/schedule", pickupHandler.Schedule)
		api.PUT("/pickups/:id/complete", pickupHandler.Complete)
		api.PUT("/pickups/:id/cancel", pickupHandler.Cancel)

		// Payment routes
		api.POST("/payments", paymentHandler.CreateOrEnsure)
		api.GET("/payments", paymentHandler.List)
		api.PUT("/payments/:id/confirm", paymentHandler.Confirm)

		// Report routes
		api.GET("/reports/waste-summary", reportHandler.WasteSummary)
		api.GET("/reports/payment-summary", reportHandler.PaymentSummary)
	}

	return router
}
