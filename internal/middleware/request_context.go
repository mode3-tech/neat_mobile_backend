package middleware

import (
	"log"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"golang.org/x/net/context"
)

const (
	RequestIDHeader     = "X-Request-ID"
	RequestIDContextKey = "request_id"
	ClientIPContextKey  = "client_ip"
)

// RequestContextLogger injects a request id and emits a single structured log record per request.
func RequestContextLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		requestID := strings.TrimSpace(c.GetHeader(RequestIDHeader))
		if requestID == "" {
			requestID = uuid.NewString()
		}

		c.Request = c.Request.WithContext(context.WithValue(c.Request.Context(), RequestIDContextKey, requestID))
		c.Set(RequestIDContextKey, requestID)
		c.Writer.Header().Set(RequestIDHeader, requestID)

		clientIP := c.ClientIP()
		c.Request = c.Request.WithContext(context.WithValue(c.Request.Context(), ClientIPContextKey, clientIP))
		c.Set(ClientIPContextKey, clientIP)

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
			clientIP,
			c.Request.UserAgent(),
			strings.TrimSpace(c.Errors.String()),
		)
	}
}

func GetRequestID(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if requestID, ok := ctx.Value(RequestIDContextKey).(string); ok {
		return requestID
	}
	return ""
}

func GetClientIP(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if clientIP, ok := ctx.Value(ClientIPContextKey).(string); ok {
		return clientIP
	}
	return ""
}
