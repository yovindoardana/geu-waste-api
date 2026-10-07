package validator

import (
	"errors"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"strings"
)

const (
	MaxFileSize     = 5 * 1024 * 1024 // 5 MiB
	MaxMultipartMem = 6 * 1024 * 1024 // 6 MiB
	MaxDimension    = 10000           // 10,000 px
	MaxTotalPixels  = 20000000        // 20 megapixels
)

// ValidateProofFile validates size, MIME type via magic bytes, and image dimensions.
func ValidateProofFile(fileHeader *multipart.FileHeader) (ext string, err error) {
	if fileHeader == nil {
		return "", errors.New("proof file is required")
	}

	if fileHeader.Size == 0 {
		return "", errors.New("proof file cannot be empty")
	}

	if fileHeader.Size > MaxFileSize {
		return "", &PayloadTooLargeError{Message: "proof file exceeds maximum allowed size of 5 MiB"}
	}

	file, err := fileHeader.Open()
	if err != nil {
		return "", fmt.Errorf("failed to open proof file: %w", err)
	}
	defer file.Close()

	// Read first 512 bytes for MIME type sniffing
	headerBytes := make([]byte, 512)
	n, err := file.Read(headerBytes)
	if err != nil && !errors.Is(err, io.EOF) {
		return "", fmt.Errorf("failed to read file header: %w", err)
	}

	mimeType := http.DetectContentType(headerBytes[:n])
	switch {
	case strings.HasPrefix(mimeType, "image/jpeg"):
		ext = ".jpg"
	case strings.HasPrefix(mimeType, "image/png"):
		ext = ".png"
	default:
		return "", &MediaTypeError{Message: fmt.Sprintf("unsupported media type: %s, only image/jpeg and image/png are allowed", mimeType)}
	}

	// Rewind to beginning to decode image config
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return "", fmt.Errorf("failed to seek file: %w", err)
	}

	cfg, format, err := image.DecodeConfig(file)
	if err != nil {
		return "", fmt.Errorf("invalid image format: %w", err)
	}

	if format != "jpeg" && format != "png" {
		return "", &MediaTypeError{Message: fmt.Sprintf("unsupported image format: %s", format)}
	}

	if cfg.Width <= 0 || cfg.Height <= 0 {
		return "", errors.New("invalid image dimensions")
	}

	if cfg.Width > MaxDimension || cfg.Height > MaxDimension {
		return "", fmt.Errorf("image dimension exceeds maximum limit of %d px (width: %d, height: %d)", MaxDimension, cfg.Width, cfg.Height)
	}

	totalPixels := int64(cfg.Width) * int64(cfg.Height)
	if totalPixels > MaxTotalPixels {
		return "", fmt.Errorf("total image pixels (%d) exceeds maximum limit of 20 megapixels", totalPixels)
	}

	return ext, nil
}

// PayloadTooLargeError represents 413 file size exceeded error.
type PayloadTooLargeError struct {
	Message string
}

func (e *PayloadTooLargeError) Error() string {
	return e.Message
}

// SanitizeFilename ensures filename is only a base filename without directory traversal.
func SanitizeFilename(filename string) string {
	return filepath.Base(filename)
}
