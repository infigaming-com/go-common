package middleware

import (
	"bytes"

	"github.com/gin-gonic/gin"
)

type responseWriter struct {
	gin.ResponseWriter
	body *bytes.Buffer
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	rw.body.Write(b)                  // Capture the response body
	return rw.ResponseWriter.Write(b) // Write to the original ResponseWriter
}
