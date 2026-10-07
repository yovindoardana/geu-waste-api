package handler

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestUploadHandler_ServeFile(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "geu_upload_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create dummy test file
	testFilename := "sample-image.png"
	testContent := []byte("dummy image content")
	if err := os.WriteFile(filepath.Join(tempDir, testFilename), testContent, 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	h := NewUploadHandler(tempDir)
	router := gin.New()
	router.GET("/uploads/payment-proofs/:filename", h.ServeFile)

	t.Run("Existing file returned 200", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/uploads/payment-proofs/"+testFilename, nil)
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", w.Code)
		}
		if w.Body.String() != string(testContent) {
			t.Errorf("unexpected body content: %s", w.Body.String())
		}
	})

	t.Run("Non-existent file returned 404", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/uploads/payment-proofs/missing.png", nil)
		router.ServeHTTP(w, req)

		if w.Code != http.StatusNotFound {
			t.Errorf("expected 404, got %d", w.Code)
		}
	})

	t.Run("Directory traversal attempt sanitized and blocked", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/uploads/payment-proofs/..%2F..%2Fetc%2Fpasswd", nil)
		router.ServeHTTP(w, req)

		if w.Code != http.StatusNotFound {
			t.Errorf("expected 404, got %d", w.Code)
		}
	})
}
