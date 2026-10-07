package handler

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/yovindoardana/geu-waste-api/internal/response"
)

// HealthHandler handles health check requests.
type HealthHandler struct {
	db *pgxpool.Pool
}

// NewHealthHandler creates a new HealthHandler.
func NewHealthHandler(db *pgxpool.Pool) *HealthHandler {
	return &HealthHandler{db: db}
}

// Health checks the service and database readiness.
func (h *HealthHandler) Health(c *gin.Context) {
	if h.db != nil {
		ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
		defer cancel()

		if err := h.db.Ping(ctx); err != nil {
			response.Error(c, http.StatusServiceUnavailable, response.CodeServiceUnavailable, "service unavailable", nil)
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "service is healthy",
	})
}
