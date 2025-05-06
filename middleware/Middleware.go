// Package middleware cung cấp các middleware cho Gin framework
// bao gồm logging, recovery, và thống kê metrics.
//
// # Cách sử dụng:
//
//	import (
//	    "github.com/gin-gonic/gin"
//	    "github.com/your-module/middleware"
//	)
//
//	func main() {
//	    r := gin.Default()
//
//	    r.Use(middleware.RecoveryMiddleware())
//	    r.Use(middleware.LogRequestMiddleware())
//	    r.Use(middleware.LogResponseMiddleware())
//
//	    r.GET("/ping", func(c *gin.Context) {
//	        c.JSON(200, gin.H{"message": "pong"})
//	    })
//
//	    r.Run()
//	}
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

// SetLogger cho phép thay thế logger mặc định.
//
// logger: một struct implement interface Logger.
func SetLogger(logger Logger) {
	defaultLogger = logger
}

// GetMetrics trả về con trỏ đến struct Metrics toàn cục
// chứa các thông tin thống kê hiện tại của hệ thống.
func GetMetrics() *Metrics {
	return metrics
}

// LogEntry đại diện cho một entry log gồm request/response
type LogEntry struct {
	StatusCode  int           // HTTP status code
	Method      string        // HTTP method
	Path        string        // URL path
	Request     string        // Request body (JSON, nếu có)
	Response    string        // Response body (JSON, nếu có)
	ProcessTime time.Duration // Thời gian xử lý request
	ClientIP    string        // Địa chỉ IP của client
	UserAgent   string        // User agent string
	RequestID   string        // UUID của request
	Error       string        // Error nếu có panic
}

// ResponseWriter là wrapper cho gin.ResponseWriter để ghi lại response body
type ResponseWriter struct {
	gin.ResponseWriter
	body        *bytes.Buffer
	statusCode  int
	wroteHeader bool
}

// Write ghi dữ liệu vào buffer và sau đó xuống response writer gốc
func (w *ResponseWriter) Write(b []byte) (int, error) {
	if !w.wroteHeader {
		w.WriteHeader(200)
	}
	w.body.Write(b)
	return w.ResponseWriter.Write(b)
}

// WriteHeader lưu status code và chỉ ghi một lần duy nhất
func (w *ResponseWriter) WriteHeader(code int) {
	if !w.wroteHeader {
		w.statusCode = code
		w.wroteHeader = true
		w.ResponseWriter.WriteHeader(code)
	}
}

// RecoveryMiddleware trả về middleware dùng để recover panic
// và log lỗi ra hệ thống đồng thời trả về lỗi HTTP 500
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

// LogRequestMiddleware trả về middleware để log thông tin request đầu vào
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

// LogResponseMiddleware trả về middleware để log thông tin response đầu ra
// và ghi nhận các metrics liên quan đến request.
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

		// Ghi lại metrics
		atomic.AddUint64(&metrics.TotalRequests, 1)
		atomic.AddUint64(&metrics.TotalDuration, uint64(duration.Milliseconds()))
		metrics.RecordRequest(c.Request.Method, bodyWriter.statusCode, duration)
	}
}

// isMultipartForm kiểm tra xem content-type có phải multipart form
func isMultipartForm(contentType string) bool {
	return strings.HasPrefix(contentType, "multipart/form-data")
}

// compactJSON nhận chuỗi JSON và loại bỏ các khoảng trắng không cần thiết
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

// formatDuration định dạng duration thành chuỗi "x.xxms"
func formatDuration(d time.Duration) string {
	return fmt.Sprintf("%.2fms", float64(d.Microseconds())/1000.0)
}
