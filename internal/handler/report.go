package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/yovindoardana/geu-waste-api/internal/response"
	"github.com/yovindoardana/geu-waste-api/internal/service"
)

// ReportHandler handles HTTP requests for reporting metrics.
type ReportHandler struct {
	svc *service.ReportService
}

// NewReportHandler creates a new ReportHandler.
func NewReportHandler(svc *service.ReportService) *ReportHandler {
	return &ReportHandler{svc: svc}
}

// WasteSummary handles GET /api/reports/waste-summary.
func (h *ReportHandler) WasteSummary(c *gin.Context) {
	report, err := h.svc.GetWasteSummary(c.Request.Context())
	if err != nil {
		response.Error(c, http.StatusInternalServerError, response.CodeInternalError, "failed to generate waste summary report", nil)
		return
	}

	response.JSON(c, http.StatusOK, "waste summary retrieved successfully", report)
}

// PaymentSummary handles GET /api/reports/payment-summary.
func (h *ReportHandler) PaymentSummary(c *gin.Context) {
	report, err := h.svc.GetPaymentSummary(c.Request.Context())
	if err != nil {
		response.Error(c, http.StatusInternalServerError, response.CodeInternalError, "failed to generate payment summary report", nil)
		return
	}

	response.JSON(c, http.StatusOK, "payment summary retrieved successfully", report)
}
