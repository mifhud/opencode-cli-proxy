package middleware

import (
	"log"
	"time"

	"github.com/gin-gonic/gin"
)

func Logger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		log.Printf("method=%s path=%s status=%d latency_ms=%d bytes=%d model=%s request_id=%v", c.Request.Method, c.FullPath(), c.Writer.Status(), time.Since(start).Milliseconds(), c.Writer.Size(), c.GetString("client_model"), c.GetString("request_id"))
	}
}
