package middleware

import (
	"bytes"
	"github.com/gin-gonic/gin"
	"github.com/kimxuanhong/go-utils/safe"
	"io"
	"strings"
	"time"
)

func RecoveryMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		safe.SafeGo(func(ex error) {
			if ex != nil {
				c.JSON(500, gin.H{
					"message": "Internal Server Error. Please try again later.",
				})
				c.Abort()
				return
			}
			c.Next()
		})
	}
}

func LogRequestMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()

		var requestBody []byte
		if c.Request.Body != nil && !isMultipartForm(c.Request.Header.Get("Content-Type")) {
			requestBody, _ = io.ReadAll(c.Request.Body)
			c.Request.Body = io.NopCloser(bytes.NewBuffer(requestBody))
		}
		entryReq := LogEntry{
			Timestamp:   time.Now(),
			StatusCode:  c.Writer.Status(),
			Method:      c.Request.Method,
			Path:        c.Request.URL.Path,
			Request:     string(requestBody),
			ProcessTime: time.Since(start),
		}
		LogReqChannel <- entryReq

		c.Next()
	}
}

func isMultipartForm(contentType string) bool {
	return strings.HasPrefix(contentType, "multipart/form-data")
}

func LogResponseMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()

		bodyWriter := &ResponseWriter{
			ResponseWriter: c.Writer,
			body:           bytes.NewBufferString(""),
		}
		c.Writer = bodyWriter

		c.Next()

		entryRes := LogEntry{
			Timestamp:   time.Now(),
			StatusCode:  c.Writer.Status(),
			Method:      c.Request.Method,
			Path:        c.Request.URL.Path,
			Response:    bodyWriter.body.String(),
			ProcessTime: time.Since(start),
		}
		LogResChannel <- entryRes
	}
}
