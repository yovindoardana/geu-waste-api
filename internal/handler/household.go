package handler

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/yovindoardana/geu-waste-api/internal/domain"
	"github.com/yovindoardana/geu-waste-api/internal/response"
	"github.com/yovindoardana/geu-waste-api/internal/service"
	"github.com/yovindoardana/geu-waste-api/internal/validator"
)

// HouseholdHandler handles HTTP requests for household resources.
type HouseholdHandler struct {
	svc *service.HouseholdService
}

// NewHouseholdHandler creates a new HouseholdHandler.
func NewHouseholdHandler(svc *service.HouseholdService) *HouseholdHandler {
	return &HouseholdHandler{svc: svc}
}

// Create handles POST /api/households.
func (h *HouseholdHandler) Create(c *gin.Context) {
	var req domain.CreateHouseholdRequest
	if err := validator.DecodeJSONStrict(c, &req); err != nil {
		var mediaErr *validator.MediaTypeError
		if errors.As(err, &mediaErr) {
			response.Error(c, http.StatusUnsupportedMediaType, response.CodeUnsupportedMediaType, mediaErr.Message, nil)
			return
		}
		validator.HandleValidationError(c, "body", err.Error())
		return
	}

	result, err := h.svc.CreateHousehold(c.Request.Context(), req)
	if err != nil {
		var valErr *service.ValidationError
		if errors.As(err, &valErr) {
			validator.HandleValidationError(c, valErr.Field, valErr.Message)
			return
		}
		response.Error(c, http.StatusInternalServerError, response.CodeInternalError, "failed to create household", nil)
		return
	}

	response.JSON(c, http.StatusCreated, "household created successfully", result)
}

// List handles GET /api/households.
func (h *HouseholdHandler) List(c *gin.Context) {
	page, limit, err := validator.ParsePagination(c)
	if err != nil {
		validator.HandleValidationError(c, "query", err.Error())
		return
	}

	items, total, err := h.svc.ListHouseholds(c.Request.Context(), page, limit)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, response.CodeInternalError, "failed to list households", nil)
		return
	}

	meta := response.CalculateMeta(page, limit, total)
	response.ListJSON(c, http.StatusOK, "households retrieved successfully", items, meta)
}

// GetByID handles GET /api/households/:id.
func (h *HouseholdHandler) GetByID(c *gin.Context) {
	id, err := validator.ParseUUID(c.Param("id"))
	if err != nil {
		validator.HandleValidationError(c, "id", err.Error())
		return
	}

	result, err := h.svc.GetHousehold(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			response.Error(c, http.StatusNotFound, response.CodeResourceNotFound, "household not found", nil)
			return
		}
		response.Error(c, http.StatusInternalServerError, response.CodeInternalError, "failed to retrieve household", nil)
		return
	}

	response.JSON(c, http.StatusOK, "household retrieved successfully", result)
}

// Delete handles DELETE /api/households/:id.
func (h *HouseholdHandler) Delete(c *gin.Context) {
	id, err := validator.ParseUUID(c.Param("id"))
	if err != nil {
		validator.HandleValidationError(c, "id", err.Error())
		return
	}

	if err := h.svc.DeleteHousehold(c.Request.Context(), id); err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			response.Error(c, http.StatusNotFound, response.CodeResourceNotFound, "household not found", nil)
			return
		}
		if errors.Is(err, domain.ErrHouseholdHasDependencies) {
			response.Error(c, http.StatusConflict, response.CodeHouseholdHasDependencies, "household has dependencies and cannot be deleted", nil)
			return
		}
		response.Error(c, http.StatusInternalServerError, response.CodeInternalError, "failed to delete household", nil)
		return
	}

	response.JSON(c, http.StatusOK, "household deleted successfully", nil)
}
