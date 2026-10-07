package handler

import (
	"net/http"
	"os"
	"path/filepath"

	"github.com/gin-gonic/gin"
	"github.com/yovindoardana/geu-waste-api/internal/response"
	"github.com/yovindoardana/geu-waste-api/internal/validator"
)

// UploadHandler handles static file serving for uploaded payment proofs.
type UploadHandler struct {
	uploadDir string
}

// NewUploadHandler creates a new UploadHandler.
func NewUploadHandler(uploadDir string) *UploadHandler {
	return &UploadHandler{uploadDir: uploadDir}
}

// ServeFile serves the uploaded image binary directly, excluding JSON envelope on success.
func (h *UploadHandler) ServeFile(c *gin.Context) {
	filename := validator.SanitizeFilename(c.Param("filename"))
	if filename == "" || filename == "." || filename == "/" {
		response.Error(c, http.StatusNotFound, response.CodeResourceNotFound, "file not found", nil)
		return
	}

	targetPath := filepath.Join(h.uploadDir, filename)
	info, err := os.Stat(targetPath)
	if err != nil {
		if os.IsNotExist(err) {
			response.Error(c, http.StatusNotFound, response.CodeResourceNotFound, "file not found", nil)
			return
		}
		response.Error(c, http.StatusInternalServerError, response.CodeInternalError, "failed to read file", nil)
		return
	}

	if info.IsDir() {
		response.Error(c, http.StatusNotFound, response.CodeResourceNotFound, "file not found", nil)
		return
	}

	http.ServeFile(c.Writer, c.Request, targetPath)
}
