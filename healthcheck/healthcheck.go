package healthcheck

import (
	"github.com/gin-gonic/gin"
	"github.com/okieraised/go-common/api_response"
	"github.com/okieraised/go-common/cerrors"
	"github.com/okieraised/go-common/constants"
	"github.com/okieraised/go-common/infrastructures/logging"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

type HealthCheckResponse struct {
	Healthy bool `json:"healthy"`
}

type Router struct {
	logger *logging.Logger
	tracer trace.Tracer
}

func NewHealthcheckRouter(logger *logging.Logger, tr trace.Tracer) *Router {
	return &Router{
		logger: logger,
		tracer: tr,
	}
}

func (r *Router) Routes(engine *gin.RouterGroup, path string) {
	routes := engine.Group(path)
	{
		routes = routes.Group("/health")
		routes.GET("", r.healthcheck)
	}
}

func (r *Router) healthcheck(ctx *gin.Context) {
	_, span := r.tracer.Start(ctx, ctx.Request.URL.Path, trace.WithAttributes(attribute.KeyValue{
		Key:   constants.APIFieldRequestID,
		Value: attribute.StringValue(ctx.GetString(constants.APIFieldRequestID)),
	}))
	defer span.End()

	resp := api_response.New[HealthCheckResponse](ctx)

	resp.Populate(
		cerrors.OK.Code,
		cerrors.OK.Message,
		HealthCheckResponse{
			Healthy: true,
		},
		nil,
		0,
	)
	ctx.JSON(cerrors.OK.HTTPStatus, resp)
	return
}
