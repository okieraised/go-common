package middlewares

import (
	"runtime/debug"

	"github.com/gin-gonic/gin"
	"github.com/okieraised/go-common/api_response"
	"github.com/okieraised/go-common/cerrors"
	"github.com/okieraised/go-common/infrastructures/logger"
)

func RecoveryMW() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				resp := api_response.New[any](ctx)
				logger.GetDefault().Error(string(debug.Stack()))
				resp.Populate(
					cerrors.ErrGenericInternalServer.Code,
					cerrors.ErrGenericInternalServer.Message,
					err,
					nil,
					nil)
				ctx.AbortWithStatusJSON(cerrors.ErrGenericInternalServer.HTTPStatus, resp)
				return
			}
		}()
		ctx.Next()
	}
}
