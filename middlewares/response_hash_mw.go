package middlewares

import (
	"github.com/gin-gonic/gin"
	"github.com/okieraised/go-common/ioutils"
)

func ResponseHashMW() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		writer := &ioutils.ResponseWriterInterceptor{
			ResponseWriter: ctx.Writer,
			Body:           make([]byte, 0),
		}
		ctx.Writer = writer
		ctx.Next()
	}
}
