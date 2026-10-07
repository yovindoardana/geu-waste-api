package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/yovindoardana/geu-waste-api/internal/response"
	"github.com/yovindoardana/geu-waste-api/internal/service"
)

func TestHouseholdHandler_InvalidRequests(t *testing.T) {
	svc := service.NewHouseholdService(nil)
	h := NewHouseholdHandler(svc)

	router := gin.New()
	router.POST("/api/households", h.Create)
	router.GET("/api/households/:id", h.GetByID)
	router.DELETE("/api/households/:id", h.Delete)

	t.Run("Create - Reject non-JSON content type", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/api/households", bytes.NewBufferString("hello"))
		req.Header.Set("Content-Type", "text/plain")
		router.ServeHTTP(w, req)

		if w.Code != http.StatusUnsupportedMediaType {
			t.Errorf("expected 415, got %d", w.Code)
		}
	})

	t.Run("Create - Reject unknown fields", func(t *testing.T) {
		w := httptest.NewRecorder()
		body := map[string]string{
			"owner_name": "Test",
			"address":    "Jalan Test",
			"extra":      "unknown field",
		}
		jsonBytes, _ := json.Marshal(body)
		req, _ := http.NewRequest("POST", "/api/households", bytes.NewBuffer(jsonBytes))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400 for unknown field, got %d", w.Code)
		}
	})

	t.Run("GetByID - Invalid UUID", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/api/households/not-a-valid-uuid", nil)
		router.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400, got %d", w.Code)
		}
		var resp response.ErrorResponse
		json.Unmarshal(w.Body.Bytes(), &resp)
		if resp.Code != response.CodeValidationError {
			t.Errorf("expected code VALIDATION_ERROR, got %s", resp.Code)
		}
	})

	t.Run("Delete - Nil UUID rejected", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("DELETE", "/api/households/00000000-0000-0000-0000-000000000000", nil)
		router.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400, got %d", w.Code)
		}
	})
}
