package middlewares

import (
	"github.com/gin-gonic/gin"
	"github.com/okieraised/go-common/api_response"
	"github.com/okieraised/go-common/cerrors"
	"github.com/okieraised/go-common/constants"
)

func ApiKeyValidationMW(expected string) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		apiKey := ctx.GetHeader(constants.HeaderXAPIKey)
		if apiKey == "" || apiKey != expected {
			resp := api_response.New[any](ctx)
			resp.Populate(
				cerrors.OK.Code,
				cerrors.OK.Message,
				nil,
				nil,
				0,
			)
			ctx.AbortWithStatusJSON(cerrors.OK.HTTPStatus, resp)
			return
		}

		ctx.Next()
	}
}
