package middlewares

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	"github.com/gin-gonic/gin"
	"github.com/okieraised/go-common/constants"
)

type ResponseWriterInterceptor struct {
	gin.ResponseWriter
	Body []byte
}

func (w *ResponseWriterInterceptor) Write(data []byte) (int, error) {
	w.Body = append(w.Body, data...)
	hash := sha256.Sum256(data)
	w.Header().Add(constants.HeaderContentDigest, fmt.Sprintf("sha256=%s", hex.EncodeToString(hash[:])))
	return w.ResponseWriter.Write(data)
}

func ResponseHashMW() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		writer := &ResponseWriterInterceptor{
			ResponseWriter: ctx.Writer,
			Body:           make([]byte, 0),
		}
		ctx.Writer = writer
		ctx.Next()
	}
}
