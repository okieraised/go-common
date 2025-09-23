package middlewares

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/okieraised/go-common/api_response"
	"github.com/okieraised/go-common/cerrors"
)

func RequestTimeoutMW(timeoutDuration time.Duration) func(ctx *gin.Context) {
	return func(ctx *gin.Context) {
		finish := make(chan struct{}, 1)
		panicChan := make(chan interface{}, 1)

		go func() {
			defer func() {
				if p := recover(); p != nil {
					panicChan <- p
				}
			}()
			ctx.Next()
			finish <- struct{}{}
		}()

		resp := api_response.New[any](ctx)
		select {
		case <-panicChan:
			resp.Populate(
				cerrors.ErrGenericInternalServer.Code,
				cerrors.ErrGenericInternalServer.Message,
				nil,
				nil,
				nil,
			)
			ctx.AbortWithStatusJSON(cerrors.ErrGenericInternalServer.HTTPStatus, resp)
			return
		case <-time.After(timeoutDuration):
			resp.Populate(
				cerrors.ErrGenericRequestTimedOut.Code,
				cerrors.ErrGenericRequestTimedOut.Message,
				nil,
				nil,
				nil,
			)
			ctx.AbortWithStatusJSON(cerrors.ErrGenericRequestTimedOut.HTTPStatus, resp)
			return
		case <-finish:
		}
	}
}
