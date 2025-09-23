package tracing

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go.opentelemetry.io/otel/trace"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.28.0"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"
)

var (
	tp   *sdktrace.TracerProvider
	once sync.Once
)

// Options configures Init.
type Options struct {
	Endpoint    string
	Insecure    bool
	ServiceName string
	Namespace   string
	Timeout     time.Duration
}

// Init initializes a process-wide tracer provider exactly once.
// It returns a Shutdown function to flush/close the provider.
func Init(opt *Options) (func(ctx context.Context) error, error) {
	var initErr error
	once.Do(func() {
		if opt.Timeout <= 0 {
			opt.Timeout = 10 * time.Second
		}
		ctx, cancel := context.WithTimeout(context.Background(), opt.Timeout)
		defer cancel()

		grpcOpts := []grpc.DialOption{
			grpc.WithKeepaliveParams(keepalive.ClientParameters{PermitWithoutStream: true}),
		}
		if opt.Insecure {
			grpcOpts = append(grpcOpts, grpc.WithTransportCredentials(insecure.NewCredentials()))
		}

		traceClient := otlptracegrpc.NewClient(
			otlptracegrpc.WithEndpoint(opt.Endpoint),
			otlptracegrpc.WithDialOption(grpcOpts...),
		)

		exp, err := otlptrace.New(ctx, traceClient)
		if err != nil {
			initErr = fmt.Errorf("create otlp trace exporter: %w", err)
			return
		}

		res, err := resource.New(ctx,
			resource.WithFromEnv(),
			resource.WithProcess(),
			resource.WithTelemetrySDK(),
			resource.WithHost(),
			resource.WithAttributes(
				semconv.ServiceName(opt.ServiceName),
				semconv.K8SNamespaceName(opt.Namespace),
			),
		)
		if err != nil {
			initErr = fmt.Errorf("create resource: %w", err)
			return
		}

		sampler := sdktrace.ParentBased(sdktrace.TraceIDRatioBased(1.0))

		tp = sdktrace.NewTracerProvider(
			sdktrace.WithResource(res),
			sdktrace.WithSampler(sampler),
			sdktrace.WithBatcher(
				exp,
				sdktrace.WithBatchTimeout(5*time.Second),
				sdktrace.WithExportTimeout(10*time.Second),
			),
		)

		otel.SetTracerProvider(tp)
		otel.SetTextMapPropagator(
			propagation.NewCompositeTextMapPropagator(
				propagation.TraceContext{},
				propagation.Baggage{},
			),
		)
	})

	if initErr != nil {
		return nil, initErr
	}
	return Shutdown, nil
}

// MustInit panics on error.
func MustInit(opt *Options) func(ctx context.Context) error {
	shutdown, err := Init(opt)
	if err != nil {
		panic(err)
	}
	return shutdown
}

func Provider() *sdktrace.TracerProvider { return tp }

func Tracer(name string, opts ...trace.TracerOption) trace.Tracer {
	if tp == nil {
		return otel.Tracer(name, opts...)
	}
	return tp.Tracer(name, opts...)
}

func Shutdown(ctx context.Context) error {
	if tp == nil {
		return nil
	}
	err := tp.Shutdown(ctx)
	tp = nil
	return err
}
