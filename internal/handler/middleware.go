package handler

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/yovindoardana/geu-waste-api/internal/response"
)

// RecoveryMiddleware handles panics cleanly and responds with 500 error envelope.
func RecoveryMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				// Log the error internally (can be extended with structured logger)
				c.Header("Content-Type", "application/json")
				response.Error(c, http.StatusInternalServerError, response.CodeInternalError, "internal server error", nil)
				c.Abort()
			}
		}()
		c.Next()
	}
}

// BodyLimitMiddleware restricts request body size:
// - multipart/form-data: max multipartMaxBytes (6 MiB)
// - json / default: max jsonMaxBytes (1 MiB)
func BodyLimitMiddleware(jsonMaxBytes, multipartMaxBytes int64) gin.HandlerFunc {
	return func(c *gin.Context) {
		maxBytes := jsonMaxBytes
		contentType := c.GetHeader("Content-Type")
		if strings.HasPrefix(strings.ToLower(contentType), "multipart/form-data") {
			maxBytes = multipartMaxBytes
		}

		if c.Request.ContentLength > maxBytes {
			response.Error(c, http.StatusRequestEntityTooLarge, response.CodePayloadTooLarge, "payload too large", nil)
			c.Abort()
			return
		}
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxBytes)
		c.Next()
	}
}

// NoRouteHandler handles 404 Not Found in JSON envelope.
func NoRouteHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		response.Error(c, http.StatusNotFound, response.CodeResourceNotFound, fmt.Sprintf("resource not found: %s %s", c.Request.Method, c.Request.URL.Path), nil)
	}
}

// NoMethodHandler handles 405 Method Not Allowed in JSON envelope.
func NoMethodHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		response.Error(c, http.StatusMethodNotAllowed, response.CodeMethodNotAllowed, fmt.Sprintf("method %s not allowed for %s", c.Request.Method, c.Request.URL.Path), nil)
	}
}
