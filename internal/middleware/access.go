package middleware

import (
	"encoding/json"
	"log"
	"strings"
	"time"

	"github.com/aapng-cmd/ctf_platform/internal/webutil"
	"github.com/gin-gonic/gin"
)

// AccessLog writes one JSON line per request to stdout. The line has no body,
// query string, cookie or header. Successful dashboard polls are skipped:
// every open board asks for that route every few seconds.
func AccessLog() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		id := webutil.RandomHex(4)
		c.Header("X-Request-Id", id)
		c.Next()
		path := c.Request.URL.Path
		if strings.HasPrefix(path, "/static/") || strings.HasPrefix(path, "/swagger") {
			return
		}
		status := c.Writer.Status()
		if c.Request.Method == "GET" && status < 400 && path == "/api/v1/ad/dashboard" {
			return
		}
		if len(path) > 512 {
			path = path[:512]
		}
		line, err := json.Marshal(accessLine{
			TS:     time.Now().UTC().Format(time.RFC3339Nano),
			ID:     id,
			Method: c.Request.Method,
			Path:   path,
			Status: status,
			MS:     time.Since(start).Milliseconds(),
			Bytes:  c.Writer.Size(),
			IP:     c.ClientIP(),
		})
		if err != nil {
			return
		}
		log.Printf("%s", line)
	}
}

type accessLine struct {
	TS     string `json:"ts"`
	ID     string `json:"id"`
	Method string `json:"method"`
	Path   string `json:"path"`
	Status int    `json:"status"`
	MS     int64  `json:"ms"`
	Bytes  int    `json:"bytes"`
	IP     string `json:"ip"`
}
