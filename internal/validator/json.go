package validator

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/yovindoardana/geu-waste-api/internal/response"
)

// DecodeJSONStrict decodes JSON from request body while rejecting unknown fields and malformed payloads.
func DecodeJSONStrict(c *gin.Context, target any) error {
	if c.Request.Body == nil {
		return errors.New("request body is empty")
	}

	contentType := c.GetHeader("Content-Type")
	if contentType == "" || (!strings.HasPrefix(contentType, "application/json") && !strings.Contains(contentType, "application/json")) {
		return &MediaTypeError{Message: "Content-Type must be application/json"}
	}

	bodyBytes, err := io.ReadAll(c.Request.Body)
	if err != nil {
		return fmt.Errorf("failed to read request body: %w", err)
	}

	trimmed := bytes.TrimSpace(bodyBytes)
	if len(trimmed) == 0 {
		return errors.New("request body cannot be empty")
	}

	// Must start with '{' for JSON object
	if !bytes.HasPrefix(trimmed, []byte("{")) || !bytes.HasSuffix(trimmed, []byte("}")) {
		return errors.New("request body must be a valid JSON object")
	}

	decoder := json.NewDecoder(bytes.NewReader(trimmed))
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(target); err != nil {
		return fmt.Errorf("invalid JSON payload: %w", err)
	}

	// Ensure no trailing tokens
	if decoder.More() {
		return errors.New("request body contains trailing data after JSON object")
	}

	return nil
}

// MediaTypeError represents unsupported media type.
type MediaTypeError struct {
	Message string
}

func (e *MediaTypeError) Error() string {
	return e.Message
}

// ParseUUID parses and validates standard UUID string.
func ParseUUID(param string) (string, error) {
	trimmed := strings.TrimSpace(param)
	if trimmed == "" {
		return "", errors.New("id is required")
	}
	parsed, err := uuid.Parse(trimmed)
	if err != nil {
		return "", fmt.Errorf("invalid UUID format: %s", param)
	}
	if parsed == uuid.Nil {
		return "", errors.New("nil UUID is not allowed")
	}
	return parsed.String(), nil
}

// ParsePagination parses page and limit query parameters.
func ParsePagination(c *gin.Context) (page, limit int, err error) {
	page = 1
	limit = 20

	pageStr := c.Query("page")
	if pageStr != "" {
		p, err := strconv.Atoi(pageStr)
		if err != nil || p < 1 {
			return 0, 0, errors.New("invalid page: must be a positive integer greater than or equal to 1")
		}
		page = p
	}

	limitStr := c.Query("limit")
	if limitStr != "" {
		l, err := strconv.Atoi(limitStr)
		if err != nil || l < 1 || l > 100 {
			return 0, 0, errors.New("invalid limit: must be an integer between 1 and 100")
		}
		limit = l
	}

	return page, limit, nil
}

// HandleValidationError writes standardized 400 validation error response.
func HandleValidationError(c *gin.Context, field, message string) {
	response.Error(c, http.StatusBadRequest, response.CodeValidationError, "validation failed", []response.FieldError{
		{Field: field, Message: message},
	})
}
