package middleware

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"sync/atomic"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/kimxuanhong/go-utils/safe"
)

var (
	defaultLogger Logger = NewDefaultLogger()
	metrics              = NewMetrics()
)

// SetLogger sets the logger to use for middleware
func SetLogger(logger Logger) {
	defaultLogger = logger
}

// LogEntry represents a log entry for both request and response
type LogEntry struct {
	StatusCode  int
	Method      string
	Path        string
	Request     string
	Response    string
	ProcessTime time.Duration
	ClientIP    string
	UserAgent   string
	RequestID   string
	Error       string
}

// ResponseWriter is a custom response writer that captures the response body
type ResponseWriter struct {
	gin.ResponseWriter
	body        *bytes.Buffer
	statusCode  int
	wroteHeader bool
}

// Write implements the io.Writer interface
func (w *ResponseWriter) Write(b []byte) (int, error) {
	if !w.wroteHeader {
		w.WriteHeader(200) // default status code
	}
	w.body.Write(b)
	return w.ResponseWriter.Write(b)
}

// WriteHeader implements the http.ResponseWriter interface
func (w *ResponseWriter) WriteHeader(code int) {
	if !w.wroteHeader {
		w.statusCode = code
		w.wroteHeader = true
		w.ResponseWriter.WriteHeader(code)
	}
}

// RecoveryMiddleware handles panic recovery
func RecoveryMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		safe.SafeGo(func(ex error) {
			if ex != nil {
				requestID := c.GetString("requestID")
				if requestID == "" {
					requestID = uuid.NewString()
				}

				defaultLogger.LogError(requestID, ex)

				c.JSON(500, gin.H{
					"message":    "Internal Server Error. Please try again later.",
					"request_id": requestID,
				})
				c.Abort()
				return
			}
			c.Next()
		})
	}
}

// LogRequestMiddleware logs incoming requests
func LogRequestMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Set("startTime", start)
		requestID := uuid.New().String()
		c.Set("requestID", requestID)

		var requestBody []byte
		if c.Request.Body != nil && !isMultipartForm(c.Request.Header.Get("Content-Type")) {
			requestBody, _ = io.ReadAll(c.Request.Body)
			c.Request.Body = io.NopCloser(bytes.NewBuffer(requestBody))
		}

		entryReq := LogEntry{
			StatusCode:  c.Writer.Status(),
			Method:      c.Request.Method,
			Path:        c.Request.URL.Path,
			Request:     compactJSON(string(requestBody)),
			ProcessTime: time.Since(start),
			ClientIP:    c.ClientIP(),
			UserAgent:   c.Request.UserAgent(),
			RequestID:   requestID,
		}
		defaultLogger.LogRequest(entryReq)

		c.Next()
	}
}

// LogResponseMiddleware logs outgoing responses
func LogResponseMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := c.GetTime("startTime")
		duration := time.Since(start)
		requestID := c.GetString("requestID")
		if requestID == "" {
			requestID = uuid.New().String()
			c.Set("requestID", requestID)
		}

		bodyWriter := &ResponseWriter{
			ResponseWriter: c.Writer,
			body:           bytes.NewBufferString(""),
		}
		c.Writer = bodyWriter

		c.Next()

		entryRes := LogEntry{
			StatusCode:  bodyWriter.statusCode,
			Method:      c.Request.Method,
			Path:        c.Request.URL.Path,
			Response:    compactJSON(bodyWriter.body.String()),
			ProcessTime: duration,
			ClientIP:    c.ClientIP(),
			UserAgent:   c.Request.UserAgent(),
			RequestID:   requestID,
		}
		defaultLogger.LogResponse(entryRes)

		// Record metrics
		atomic.AddUint64(&metrics.TotalRequests, 1)
		atomic.AddUint64(&metrics.TotalDuration, uint64(duration.Milliseconds()))
		metrics.RecordRequest(c.Request.Method, bodyWriter.statusCode, duration)
	}
}

// Helper functions
func isMultipartForm(contentType string) bool {
	return strings.HasPrefix(contentType, "multipart/form-data")
}

func compactJSON(data string) string {
	if data == "" {
		return ""
	}
	var compactedJSON bytes.Buffer
	err := json.Compact(&compactedJSON, []byte(data))
	if err != nil {
		return data
	}
	return compactedJSON.String()
}

func formatDuration(d time.Duration) string {
	return fmt.Sprintf("%.2fms", float64(d.Microseconds())/1000.0)
}
