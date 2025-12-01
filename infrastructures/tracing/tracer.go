package tracing

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/okieraised/go-common/cerrors"
	"github.com/pkg/errors"
	"go.opentelemetry.io/otel/codes"
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

type Option func(*Options)

func WithEndpoint(ep string) Option {
	return func(o *Options) { o.Endpoint = ep }
}

func WithInsecure(insecure bool) Option {
	return func(o *Options) { o.Insecure = insecure }
}

func WithServiceName(name string) Option {
	return func(o *Options) { o.ServiceName = name }
}

func WithNamespace(ns string) Option {
	return func(o *Options) { o.Namespace = ns }
}

func WithTimeout(d time.Duration) Option {
	return func(o *Options) { o.Timeout = d }
}

func NewTracerClient(opts ...Option) (func(ctx context.Context) error, error) {
	var initErr error
	once.Do(func() {
		opt := Options{}
		for _, o := range opts {
			o(&opt)
		}

		if opt.Timeout <= 0 {
			opt.Timeout = 10 * time.Second
		}
		ctx, cancel := context.WithTimeout(context.Background(), opt.Timeout)
		defer cancel()

		grpcOpts := []grpc.DialOption{
			grpc.WithKeepaliveParams(keepalive.ClientParameters{
				Time:                10 * time.Second,
				Timeout:             20 * time.Second,
				PermitWithoutStream: true,
			}),
		}
		if opt.Insecure {
			grpcOpts = append(grpcOpts, grpc.WithTransportCredentials(insecure.NewCredentials()))
		}
		traceOpts := []otlptracegrpc.Option{
			otlptracegrpc.WithEndpoint(opt.Endpoint),
			otlptracegrpc.WithDialOption(grpcOpts...),
		}

		if opt.Insecure {
			traceOpts = append(traceOpts, otlptracegrpc.WithInsecure())
		}

		traceClient := otlptracegrpc.NewClient(traceOpts...)

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

// MustTracerClient panics on error.
func MustTracerClient(opts ...Option) func(ctx context.Context) error {
	shutdown, err := NewTracerClient(opts...)
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

func normalizeError(err error) error {
	if err == nil {
		return nil
	}

	var appErr *cerrors.AppError
	if errors.As(err, &appErr) && appErr == nil {
		return nil
	}
	return err
}

func SpanWithResultErr[T any](ctx context.Context, tracer trace.Tracer, name string, fn func(context.Context) (T, error)) (T, error) {
	ctx, span := tracer.Start(ctx, name)
	defer span.End()

	result, err := fn(ctx)
	err = normalizeError(err)

	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	} else {
		span.SetStatus(codes.Ok, "")
	}

	return result, err
}
