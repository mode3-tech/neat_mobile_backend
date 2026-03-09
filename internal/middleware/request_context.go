package middleware

import (
	"log"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const (
	RequestIDHeader     = "X-Request-ID"
	RequestIDContextKey = "request_id"
)

// RequestContextLogger injects a request id and emits a single structured log record per request.
func RequestContextLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		requestID := strings.TrimSpace(c.GetHeader(RequestIDHeader))
		if requestID == "" {
			requestID = uuid.NewString()
		}

		c.Set(RequestIDContextKey, requestID)
		c.Writer.Header().Set(RequestIDHeader, requestID)

		start := time.Now()
		c.Next()

		path := c.FullPath()
		if strings.TrimSpace(path) == "" {
			path = c.Request.URL.Path
		}

		status := c.Writer.Status()
		level := "INFO"
		switch {
		case status >= 500:
			level = "ERROR"
		case status >= 400:
			level = "WARN"
		}

		log.Printf(
			"level=%s request_id=%s method=%s path=%s status=%d latency_ms=%d client_ip=%s user_agent=%q errors=%q",
			level,
			requestID,
			c.Request.Method,
			path,
			status,
			time.Since(start).Milliseconds(),
			c.ClientIP(),
			c.Request.UserAgent(),
			strings.TrimSpace(c.Errors.String()),
		)
	}
}
