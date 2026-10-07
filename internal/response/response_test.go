package response

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func TestCalculateMeta(t *testing.T) {
	tests := []struct {
		name      string
		page      int
		limit     int
		total     int64
		wantPages int
	}{
		{name: "empty items", page: 1, limit: 20, total: 0, wantPages: 0},
		{name: "exact single page", page: 1, limit: 20, total: 20, wantPages: 1},
		{name: "one item second page", page: 1, limit: 20, total: 21, wantPages: 2},
		{name: "large total", page: 2, limit: 10, total: 95, wantPages: 10},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			meta := CalculateMeta(tt.page, tt.limit, tt.total)
			if meta.TotalPages != tt.wantPages {
				t.Errorf("CalculateMeta() total_pages = %d, want %d", meta.TotalPages, tt.wantPages)
			}
			if meta.Page != tt.page {
				t.Errorf("CalculateMeta() page = %d, want %d", meta.Page, tt.page)
			}
			if meta.Limit != tt.limit {
				t.Errorf("CalculateMeta() limit = %d, want %d", meta.Limit, tt.limit)
			}
			if meta.Total != tt.total {
				t.Errorf("CalculateMeta() total = %d, want %d", meta.Total, tt.total)
			}
		})
	}
}

func TestJSONHelpers(t *testing.T) {
	t.Run("Success JSON", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)

		JSON(c, http.StatusOK, "resource created", map[string]string{"id": "123"})

		if w.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", w.Code)
		}

		var resp SuccessResponse
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if !resp.Success {
			t.Errorf("expected success true, got false")
		}
		if resp.Message != "resource created" {
			t.Errorf("expected message 'resource created', got %q", resp.Message)
		}
	})

	t.Run("List JSON", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)

		meta := CalculateMeta(1, 20, 1)
		ListJSON(c, http.StatusOK, "list fetched", []string{"item1"}, meta)

		if w.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", w.Code)
		}

		var resp ListSuccessResponse
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if !resp.Success || resp.Meta.Total != 1 {
			t.Errorf("unexpected list response: %+v", resp)
		}
	})

	t.Run("Error JSON", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)

		Error(c, http.StatusBadRequest, CodeValidationError, "validation failed", []FieldError{
			{Field: "name", Message: "name is required"},
		})

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected status 400, got %d", w.Code)
		}

		var resp ErrorResponse
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if resp.Success {
			t.Errorf("expected success false, got true")
		}
		if resp.Code != CodeValidationError {
			t.Errorf("expected code %s, got %s", CodeValidationError, resp.Code)
		}
		if len(resp.Errors) != 1 || resp.Errors[0].Field != "name" {
			t.Errorf("unexpected errors payload: %+v", resp.Errors)
		}
	})
}
