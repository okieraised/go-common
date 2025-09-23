package middlewares

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/okieraised/go-common/constants"
)

func RequestIDMW() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		requestID := uuid.New().String()
		ctx.Request.Header.Set(constants.APIFieldRequestID, requestID)
		ctx.Set(constants.APIFieldRequestID, requestID)
		ctx.Next()
	}
}
