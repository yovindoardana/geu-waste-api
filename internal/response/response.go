package response

import (
	"github.com/gin-gonic/gin"
)

// SuccessResponse defines the standard success JSON envelope.
type SuccessResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Data    any    `json:"data"`
}

// Meta defines pagination metadata.
type Meta struct {
	Page       int   `json:"page"`
	Limit      int   `json:"limit"`
	Total      int64 `json:"total"`
	TotalPages int   `json:"total_pages"`
}

// ListSuccessResponse defines the standard list success JSON envelope with pagination meta.
type ListSuccessResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Data    any    `json:"data"`
	Meta    Meta   `json:"meta"`
}

// FieldError represents a single field validation error.
type FieldError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

// ErrorResponse defines the standard error JSON envelope.
type ErrorResponse struct {
	Success bool         `json:"success"`
	Message string       `json:"message"`
	Code    string       `json:"code"`
	Errors  []FieldError `json:"errors"`
}

// JSON renders a standard success JSON response.
func JSON(c *gin.Context, statusCode int, message string, data any) {
	c.JSON(statusCode, SuccessResponse{
		Success: true,
		Message: message,
		Data:    data,
	})
}

// ListJSON renders a standard list success JSON response with pagination meta.
func ListJSON(c *gin.Context, statusCode int, message string, data any, meta Meta) {
	c.JSON(statusCode, ListSuccessResponse{
		Success: true,
		Message: message,
		Data:    data,
		Meta:    meta,
	})
}

// Error renders a standard error JSON response.
func Error(c *gin.Context, statusCode int, code, message string, fieldErrors []FieldError) {
	c.JSON(statusCode, ErrorResponse{
		Success: false,
		Message: message,
		Code:    code,
		Errors:  fieldErrors,
	})
}

// CalculateMeta calculates total pages and ensures valid pagination structure.
func CalculateMeta(page, limit int, total int64) Meta {
	totalPages := 0
	if limit > 0 && total > 0 {
		totalPages = int((total + int64(limit) - 1) / int64(limit))
	}
	return Meta{
		Page:       page,
		Limit:      limit,
		Total:      total,
		TotalPages: totalPages,
	}
}
