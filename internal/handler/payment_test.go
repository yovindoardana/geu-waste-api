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

func TestPaymentHandler_InvalidRequests(t *testing.T) {
	svc := service.NewPaymentService(nil, nil)
	h := NewPaymentHandler(svc, "/tmp/uploads")

	router := gin.New()
	router.POST("/api/payments", h.CreateOrEnsure)
	router.GET("/api/payments", h.List)
	router.PUT("/api/payments/:id/confirm", h.Confirm)

	t.Run("Create - Non-JSON content type", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/api/payments", bytes.NewBufferString("hello"))
		req.Header.Set("Content-Type", "text/plain")
		router.ServeHTTP(w, req)

		if w.Code != http.StatusUnsupportedMediaType {
			t.Errorf("expected 415, got %d", w.Code)
		}
	})

	t.Run("Create - Unknown field rejected", func(t *testing.T) {
		w := httptest.NewRecorder()
		body := map[string]string{
			"household_id": "10000000-0000-4000-8000-000000000001",
			"waste_id":     "20000000-0000-4000-8000-000000000001",
			"amount":       "50000.00",
			"extra_field":  "disallowed",
		}
		jsonBytes, _ := json.Marshal(body)
		req, _ := http.NewRequest("POST", "/api/payments", bytes.NewBuffer(jsonBytes))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400 for unknown field, got %d", w.Code)
		}
	})

	t.Run("List - Invalid start_date format", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/api/payments?start_date=2026/10/01", nil)
		router.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400 for invalid date format, got %d", w.Code)
		}
	})

	t.Run("List - start_date after end_date", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/api/payments?start_date=2026-10-10&end_date=2026-10-01", nil)
		router.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400 when start_date > end_date, got %d", w.Code)
		}
	})

	t.Run("Confirm - Invalid UUID", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("PUT", "/api/payments/invalid-uuid/confirm", nil)
		router.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400 for invalid UUID, got %d", w.Code)
		}
	})
}
