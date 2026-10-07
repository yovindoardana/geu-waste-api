package handler

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/yovindoardana/geu-waste-api/internal/domain"
	"github.com/yovindoardana/geu-waste-api/internal/response"
	"github.com/yovindoardana/geu-waste-api/internal/service"
	"github.com/yovindoardana/geu-waste-api/internal/validator"
)

// PickupHandler handles HTTP requests for waste pickup resources.
type PickupHandler struct {
	svc *service.PickupService
}

// NewPickupHandler creates a new PickupHandler.
func NewPickupHandler(svc *service.PickupService) *PickupHandler {
	return &PickupHandler{svc: svc}
}

// Create handles POST /api/pickups.
func (h *PickupHandler) Create(c *gin.Context) {
	var req domain.CreatePickupRequest
	if err := validator.DecodeJSONStrict(c, &req); err != nil {
		var mediaErr *validator.MediaTypeError
		if errors.As(err, &mediaErr) {
			response.Error(c, http.StatusUnsupportedMediaType, response.CodeUnsupportedMediaType, mediaErr.Message, nil)
			return
		}
		validator.HandleValidationError(c, "body", err.Error())
		return
	}

	result, err := h.svc.CreatePickup(c.Request.Context(), req)
	if err != nil {
		var valErr *service.ValidationError
		if errors.As(err, &valErr) {
			validator.HandleValidationError(c, valErr.Field, valErr.Message)
			return
		}
		if errors.Is(err, domain.ErrNotFound) {
			response.Error(c, http.StatusNotFound, response.CodeResourceNotFound, "household not found", nil)
			return
		}
		if errors.Is(err, domain.ErrHouseholdPendingPayment) {
			response.Error(c, http.StatusConflict, response.CodeHouseholdPendingPayment, "household has pending payment", nil)
			return
		}
		response.Error(c, http.StatusInternalServerError, response.CodeInternalError, "failed to create pickup", nil)
		return
	}

	response.JSON(c, http.StatusCreated, "pickup created successfully", result)
}

// List handles GET /api/pickups.
func (h *PickupHandler) List(c *gin.Context) {
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

	filter := domain.PickupFilter{
		Page:        page,
		Limit:       limit,
		Status:      statusPtr,
		HouseholdID: householdIDPtr,
	}

	items, total, err := h.svc.ListPickups(c.Request.Context(), filter)
	if err != nil {
		var valErr *service.ValidationError
		if errors.As(err, &valErr) {
			validator.HandleValidationError(c, valErr.Field, valErr.Message)
			return
		}
		response.Error(c, http.StatusInternalServerError, response.CodeInternalError, "failed to list pickups", nil)
		return
	}

	meta := response.CalculateMeta(page, limit, total)
	response.ListJSON(c, http.StatusOK, "pickups retrieved successfully", items, meta)
}

// Schedule handles PUT /api/pickups/:id/schedule.
func (h *PickupHandler) Schedule(c *gin.Context) {
	id, err := validator.ParseUUID(c.Param("id"))
	if err != nil {
		validator.HandleValidationError(c, "id", err.Error())
		return
	}

	var req domain.SchedulePickupRequest
	if err := validator.DecodeJSONStrict(c, &req); err != nil {
		var mediaErr *validator.MediaTypeError
		if errors.As(err, &mediaErr) {
			response.Error(c, http.StatusUnsupportedMediaType, response.CodeUnsupportedMediaType, mediaErr.Message, nil)
			return
		}
		validator.HandleValidationError(c, "body", err.Error())
		return
	}

	result, err := h.svc.SchedulePickup(c.Request.Context(), id, req)
	if err != nil {
		var valErr *service.ValidationError
		if errors.As(err, &valErr) {
			validator.HandleValidationError(c, valErr.Field, valErr.Message)
			return
		}
		if errors.Is(err, domain.ErrNotFound) {
			response.Error(c, http.StatusNotFound, response.CodeResourceNotFound, "pickup not found", nil)
			return
		}
		if errors.Is(err, domain.ErrInvalidStateTransition) {
			response.Error(c, http.StatusConflict, response.CodeInvalidStateTransition, "cannot schedule non-pending pickup", nil)
			return
		}
		if errors.Is(err, domain.ErrSafetyCheckRequired) {
			response.Error(c, http.StatusConflict, response.CodeSafetyCheckRequired, "safety check is required for electronic waste", nil)
			return
		}
		response.Error(c, http.StatusInternalServerError, response.CodeInternalError, "failed to schedule pickup", nil)
		return
	}

	response.JSON(c, http.StatusOK, "pickup scheduled successfully", result)
}

// Cancel handles PUT /api/pickups/:id/cancel.
func (h *PickupHandler) Cancel(c *gin.Context) {
	id, err := validator.ParseUUID(c.Param("id"))
	if err != nil {
		validator.HandleValidationError(c, "id", err.Error())
		return
	}

	// Validate body is empty or valid JSON {} if Content-Type is provided
	if c.Request.Body != nil && c.Request.ContentLength > 0 {
		var emptyBody map[string]any
		if err := validator.DecodeJSONStrict(c, &emptyBody); err != nil {
			validator.HandleValidationError(c, "body", err.Error())
			return
		}
		if len(emptyBody) > 0 {
			validator.HandleValidationError(c, "body", "cancel endpoint does not accept body parameters")
			return
		}
	}

	result, err := h.svc.CancelPickup(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			response.Error(c, http.StatusNotFound, response.CodeResourceNotFound, "pickup not found", nil)
			return
		}
		if errors.Is(err, domain.ErrInvalidStateTransition) {
			response.Error(c, http.StatusConflict, response.CodeInvalidStateTransition, "cannot cancel completed or already canceled pickup", nil)
			return
		}
		response.Error(c, http.StatusInternalServerError, response.CodeInternalError, "failed to cancel pickup", nil)
		return
	}

	response.JSON(c, http.StatusOK, "pickup canceled successfully", result)
}
