package middleware

import (
	"bytes"
	"encoding/json"
	"github.com/gin-gonic/gin"
	"log"
	"time"
)

type LogEntry struct {
	Timestamp   time.Time
	StatusCode  int
	Method      string
	Path        string
	Request     string
	Response    string
	ProcessTime time.Duration
}

var LogReqChannel = make(chan LogEntry, 100)
var LogResChannel = make(chan LogEntry, 100)

func OnStopServer() {
	close(LogReqChannel)
	close(LogResChannel)
}

func LogWorker() {
	go func() {
		for entry := range LogReqChannel {
			log.Printf("[%s] %s %s - %d in %v,\nRequest: %s \n",
				entry.Timestamp.Format(time.RFC3339),
				entry.Method, entry.Path,
				entry.StatusCode,
				entry.ProcessTime,
				compactJSON(entry.Request),
			)
		}
	}()

	go func() {
		for entry := range LogResChannel {
			log.Printf("[%s] %s %s - %d in %v,\nResponse: %s\n",
				entry.Timestamp.Format(time.RFC3339),
				entry.Method, entry.Path,
				entry.StatusCode,
				entry.ProcessTime,
				compactJSON(entry.Response),
			)
		}
	}()
}

type ResponseWriter struct {
	gin.ResponseWriter
	body *bytes.Buffer
}

func (w *ResponseWriter) Write(b []byte) (int, error) {
	w.body.Write(b)
	return w.ResponseWriter.Write(b)
}

func compactJSON(data string) string {
	var compactedJSON bytes.Buffer
	err := json.Compact(&compactedJSON, []byte(data))
	if err != nil {
		return data
	}
	return compactedJSON.String()
}
