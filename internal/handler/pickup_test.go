package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/yovindoardana/geu-waste-api/internal/service"
)

func TestPickupHandler_InvalidRequests(t *testing.T) {
	svc := service.NewPickupService(nil)
	h := NewPickupHandler(svc)

	router := gin.New()
	router.POST("/api/pickups", h.Create)
	router.PUT("/api/pickups/:id/schedule", h.Schedule)
	router.PUT("/api/pickups/:id/cancel", h.Cancel)

	t.Run("Create - Reject non-JSON content type", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/api/pickups", bytes.NewBufferString("hello"))
		req.Header.Set("Content-Type", "text/plain")
		router.ServeHTTP(w, req)

		if w.Code != http.StatusUnsupportedMediaType {
			t.Errorf("expected 415, got %d", w.Code)
		}
	})

	t.Run("Schedule - Invalid UUID parameter", func(t *testing.T) {
		w := httptest.NewRecorder()
		body := map[string]string{
			"pickup_date": "2026-10-08T09:00:00+07:00",
		}
		jsonBytes, _ := json.Marshal(body)
		req, _ := http.NewRequest("PUT", "/api/pickups/invalid-uuid/schedule", bytes.NewBuffer(jsonBytes))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400 for invalid UUID, got %d", w.Code)
		}
	})

	t.Run("Cancel - Reject unexpected body parameters", func(t *testing.T) {
		w := httptest.NewRecorder()
		body := map[string]string{
			"reason": "changed mind",
		}
		jsonBytes, _ := json.Marshal(body)
		req, _ := http.NewRequest("PUT", "/api/pickups/20000000-0000-4000-8000-000000000001/cancel", bytes.NewBuffer(jsonBytes))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400 for cancel with body parameters, got %d", w.Code)
		}
	})
}
