package handler

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/yovindoardana/geu-waste-api/internal/domain"
	"github.com/yovindoardana/geu-waste-api/internal/response"
	"github.com/yovindoardana/geu-waste-api/internal/service"
	"github.com/yovindoardana/geu-waste-api/internal/validator"
)

// PaymentHandler handles HTTP requests for payment resources.
type PaymentHandler struct {
	svc       *service.PaymentService
	uploadDir string
}

// NewPaymentHandler creates a new PaymentHandler.
func NewPaymentHandler(svc *service.PaymentService, uploadDir string) *PaymentHandler {
	return &PaymentHandler{
		svc:       svc,
		uploadDir: uploadDir,
	}
}

// CreateOrEnsure handles POST /api/payments (D01).
func (h *PaymentHandler) CreateOrEnsure(c *gin.Context) {
	var req domain.CreatePaymentRequest
	if err := validator.DecodeJSONStrict(c, &req); err != nil {
		var mediaErr *validator.MediaTypeError
		if errors.As(err, &mediaErr) {
			response.Error(c, http.StatusUnsupportedMediaType, response.CodeUnsupportedMediaType, mediaErr.Message, nil)
			return
		}
		validator.HandleValidationError(c, "body", err.Error())
		return
	}

	payment, isCreated, err := h.svc.EnsurePayment(c.Request.Context(), req)
	if err != nil {
		var valErr *service.ValidationError
		if errors.As(err, &valErr) {
			validator.HandleValidationError(c, valErr.Field, valErr.Message)
			return
		}
		if errors.Is(err, domain.ErrNotFound) {
			response.Error(c, http.StatusNotFound, response.CodeResourceNotFound, "pickup or household not found", nil)
			return
		}
		if errors.Is(err, domain.ErrInvalidStateTransition) {
			response.Error(c, http.StatusConflict, response.CodeInvalidStateTransition, "pickup is not completed", nil)
			return
		}
		if errors.Is(err, domain.ErrPaymentConflict) {
			response.Error(c, http.StatusConflict, response.CodePaymentConflict, "payment conflict with existing invoice", nil)
			return
		}
		response.Error(c, http.StatusInternalServerError, response.CodeInternalError, "failed to process payment", nil)
		return
	}

	if isCreated {
		response.JSON(c, http.StatusCreated, "payment created successfully", payment)
	} else {
		response.JSON(c, http.StatusOK, "payment already exists", payment)
	}
}

// List handles GET /api/payments.
func (h *PaymentHandler) List(c *gin.Context) {
	page, limit, err := validator.ParsePagination(c)
	if err != nil {
		validator.HandleValidationError(c, "query", err.Error())
		return
	}

	var statusPtr *string
	if status := strings.TrimSpace(c.Query("status")); status != "" {
		statusPtr = &status
	}

	var householdIDPtr *string
	if householdID := strings.TrimSpace(c.Query("household_id")); householdID != "" {
		householdIDPtr = &householdID
	}

	var startDatePtr, endDatePtr *time.Time
	if startStr := strings.TrimSpace(c.Query("start_date")); startStr != "" {
		t, err := time.Parse("2006-01-02", startStr)
		if err != nil {
			validator.HandleValidationError(c, "start_date", "start_date must be in YYYY-MM-DD format")
			return
		}
		utcStart := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
		startDatePtr = &utcStart
	}

	if endStr := strings.TrimSpace(c.Query("end_date")); endStr != "" {
		t, err := time.Parse("2006-01-02", endStr)
		if err != nil {
			validator.HandleValidationError(c, "end_date", "end_date must be in YYYY-MM-DD format")
			return
		}
		// End date is inclusive of the entire day up to next day 00:00:00 UTC
		utcEnd := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC).AddDate(0, 0, 1)
		endDatePtr = &utcEnd
	}

	if startDatePtr != nil && endDatePtr != nil && startDatePtr.After(*endDatePtr) {
		validator.HandleValidationError(c, "start_date", "start_date cannot be after end_date")
		return
	}

	filter := domain.PaymentFilter{
		Page:        page,
		Limit:       limit,
		Status:      statusPtr,
		HouseholdID: householdIDPtr,
		StartDate:   startDatePtr,
		EndDate:     endDatePtr,
	}

	items, total, err := h.svc.ListPayments(c.Request.Context(), filter)
	if err != nil {
		var valErr *service.ValidationError
		if errors.As(err, &valErr) {
			validator.HandleValidationError(c, valErr.Field, valErr.Message)
			return
		}
		response.Error(c, http.StatusInternalServerError, response.CodeInternalError, "failed to list payments", nil)
		return
	}

	meta := response.CalculateMeta(page, limit, total)
	response.ListJSON(c, http.StatusOK, "payments retrieved successfully", items, meta)
}

// Confirm handles PUT /api/payments/:id/confirm (BR05, D11, D12).
func (h *PaymentHandler) Confirm(c *gin.Context) {
	id, err := validator.ParseUUID(c.Param("id"))
	if err != nil {
		validator.HandleValidationError(c, "id", err.Error())
		return
	}

	// Limit multipart request size to MaxMultipartMem
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, validator.MaxMultipartMem)

	// Parse multipart form
	if err := c.Request.ParseMultipartForm(validator.MaxMultipartMem); err != nil {
		if strings.Contains(err.Error(), "too large") || strings.Contains(err.Error(), "request body too large") {
			response.Error(c, http.StatusRequestEntityTooLarge, response.CodePayloadTooLarge, "payload too large: multipart payload exceeds 6 MiB limit", nil)
			return
		}
		validator.HandleValidationError(c, "body", "invalid multipart form data")
		return
	}

	// Reject form if it contains fields other than "proof"
	for key := range c.Request.MultipartForm.Value {
		validator.HandleValidationError(c, key, "confirm endpoint does not accept form fields, only proof file")
		return
	}
	for key := range c.Request.MultipartForm.File {
		if key != "proof" {
			validator.HandleValidationError(c, key, "only proof file is accepted")
			return
		}
	}

	fileHeaders := c.Request.MultipartForm.File["proof"]
	if len(fileHeaders) == 0 {
		validator.HandleValidationError(c, "proof", "proof file is required")
		return
	}
	if len(fileHeaders) > 1 {
		validator.HandleValidationError(c, "proof", "only exactly one proof file is allowed")
		return
	}

	confirmed, err := h.svc.ConfirmPaymentWithProof(c.Request.Context(), id, fileHeaders[0], h.uploadDir)
	if err != nil {
		var payloadErr *validator.PayloadTooLargeError
		if errors.As(err, &payloadErr) {
			response.Error(c, http.StatusRequestEntityTooLarge, response.CodePayloadTooLarge, payloadErr.Message, nil)
			return
		}

		var mediaErr *validator.MediaTypeError
		if errors.As(err, &mediaErr) {
			response.Error(c, http.StatusUnsupportedMediaType, response.CodeUnsupportedMediaType, mediaErr.Message, nil)
			return
		}

		var valErr *service.ValidationError
		if errors.As(err, &valErr) {
			validator.HandleValidationError(c, valErr.Field, valErr.Message)
			return
		}

		if errors.Is(err, domain.ErrNotFound) {
			response.Error(c, http.StatusNotFound, response.CodeResourceNotFound, "payment not found", nil)
			return
		}

		if errors.Is(err, domain.ErrInvalidStateTransition) {
			response.Error(c, http.StatusConflict, response.CodeInvalidStateTransition, "cannot confirm non-pending payment", nil)
			return
		}

		// Generic validation failure on image decoding/dimensions
		if strings.Contains(err.Error(), "image") || strings.Contains(err.Error(), "dimension") || strings.Contains(err.Error(), "pixel") || strings.Contains(err.Error(), "empty") {
			validator.HandleValidationError(c, "proof", err.Error())
			return
		}

		response.Error(c, http.StatusInternalServerError, response.CodeInternalError, "failed to confirm payment", nil)
		return
	}

	response.JSON(c, http.StatusOK, "payment confirmed successfully", confirmed)
}

