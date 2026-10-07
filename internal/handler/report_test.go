package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/yovindoardana/geu-waste-api/internal/service"
)

func TestReportHandler(t *testing.T) {
	svc := service.NewReportService(nil)
	h := NewReportHandler(svc)

	router := gin.New()
	router.GET("/api/reports/waste-summary", h.WasteSummary)
	router.GET("/api/reports/payment-summary", h.PaymentSummary)

	t.Run("WasteSummary without repo returns 500", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/api/reports/waste-summary", nil)
		router.ServeHTTP(w, req)

		if w.Code != http.StatusInternalServerError {
			t.Errorf("expected 500, got %d", w.Code)
		}
		var resp map[string]any
		json.Unmarshal(w.Body.Bytes(), &resp)
		if resp["success"] != false {
			t.Errorf("expected success false, got %v", resp["success"])
		}
	})

	t.Run("PaymentSummary without repo returns 500", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/api/reports/payment-summary", nil)
		router.ServeHTTP(w, req)

		if w.Code != http.StatusInternalServerError {
			t.Errorf("expected 500, got %d", w.Code)
		}
		var resp map[string]any
		json.Unmarshal(w.Body.Bytes(), &resp)
		if resp["success"] != false {
			t.Errorf("expected success false, got %v", resp["success"])
		}
	})
}
