package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/yovindoardana/geu-waste-api/internal/response"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func TestRecoveryMiddleware(t *testing.T) {
	router := gin.New()
	router.Use(RecoveryMiddleware())
	router.GET("/panic", func(c *gin.Context) {
		panic("simulated unexpected crash")
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/panic", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 500, got %d", w.Code)
	}

	var resp response.ErrorResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Code != response.CodeInternalError {
		t.Errorf("expected code INTERNAL_ERROR, got %s", resp.Code)
	}
	if resp.Message != "internal server error" {
		t.Errorf("expected message 'internal server error', got %s", resp.Message)
	}
}

func TestBodyLimitMiddleware(t *testing.T) {
	router := gin.New()
	router.Use(BodyLimitMiddleware(100)) // 100 bytes limit
	router.POST("/upload", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	// Case 1: Body larger than limit
	largePayload := strings.Repeat("A", 200)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/upload", strings.NewReader(largePayload))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	if w.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("expected status 413, got %d", w.Code)
	}
}

func TestNoRouteAndNoMethodHandlers(t *testing.T) {
	router := gin.New()
	router.HandleMethodNotAllowed = true
	router.NoRoute(NoRouteHandler())
	router.NoMethod(NoMethodHandler())

	router.GET("/api/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	t.Run("404 Not Found", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/api/unknown-endpoint", nil)
		router.ServeHTTP(w, req)

		if w.Code != http.StatusNotFound {
			t.Errorf("expected status 404, got %d", w.Code)
		}

		var resp response.ErrorResponse
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to decode error: %v", err)
		}
		if resp.Code != response.CodeResourceNotFound {
			t.Errorf("expected code RESOURCE_NOT_FOUND, got %s", resp.Code)
		}
	})

	t.Run("405 Method Not Allowed", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/api/test", nil)
		router.ServeHTTP(w, req)

		if w.Code != http.StatusMethodNotAllowed {
			t.Errorf("expected status 405, got %d", w.Code)
		}

		var resp response.ErrorResponse
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to decode error: %v", err)
		}
		if resp.Code != response.CodeMethodNotAllowed {
			t.Errorf("expected code METHOD_NOT_ALLOWED, got %s", resp.Code)
		}
	})
}

func TestHealthHandlerWithoutDB(t *testing.T) {
	router := gin.New()
	h := NewHealthHandler(nil)
	router.GET("/health", h.Health)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/health", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp["success"] != true || resp["message"] != "service is healthy" {
		t.Errorf("unexpected health response: %+v", resp)
	}
}
