package cli

import (
	"context"
	"fmt"
	"strings"
	"time"

	grpcZap "github.com/grpc-ecosystem/go-grpc-middleware/logging/zap"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.10.0"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"

	"github.com/krzko/otelgen/internal/traces"

	"github.com/urfave/cli/v2"
	"go.uber.org/zap"
)

func genTracesCommand() *cli.Command {
	return &cli.Command{
		Name:    "traces",
		Usage:   "Generate traces",
		Aliases: []string{"t"},
		Subcommands: []*cli.Command{
			{
				Name:    "single",
				Usage:   "generate a single trace",
				Aliases: []string{"s"},
				Flags: []cli.Flag{
					&cli.BoolFlag{
						Name:    "marshal",
						Aliases: []string{"m"},
						Usage:   "marshal trace context via HTTP headers",
						Value:   false,
					},
					&cli.StringFlag{
						Name:    "scenario",
						Aliases: []string{"s"},
						Usage:   "The trace scenario to simulate (basic, eventing, microservices, web_mobile)",
						Value:   "basic",
					},
				},
				Action: func(c *cli.Context) error {
					return generateTraces(c, true)
				},
			},
			{
				Name:    "multi",
				Usage:   "generate multiple traces",
				Aliases: []string{"m"},
				Flags: []cli.Flag{
					&cli.StringSliceFlag{
						Name:    "scenarios",
						Aliases: []string{"s"},
						Usage:   "The trace scenarios to simulate (basic, web_request, mobile_request, event_driven, pub_sub, microservices, database_operation)",
						Value:   cli.NewStringSlice("basic"),
					},
					&cli.IntFlag{
						Name:    "number-traces",
						Aliases: []string{"t"},
						Usage:   "number of traces to generate in each worker",
						Value:   3,
					},
					&cli.IntFlag{
						Name:    "workers",
						Aliases: []string{"w"},
						Usage:   "number of workers (goroutines) to run",
						Value:   1,
					},
					&cli.IntFlag{
						Name:  "exporter-shards",
						Usage: "number of independent trace exporters/providers to shard across",
						Value: 1,
					},
				},
				Action: func(c *cli.Context) error {
					return generateTraces(c, false)
				},
			},
		},
	}
}

func generateTraces(c *cli.Context, isSingle bool) error {
	if err := validateOTLPEndpointFlags(c); err != nil {
		return err
	}

	tracesCfg := newTracesConfig(c)

	if isSingle {
		tracesCfg.NumTraces = 1
		tracesCfg.WorkerCount = 1
		tracesCfg.ExporterShards = 1
		tracesCfg.Scenarios = []string{c.String("scenario")}
		tracesCfg.PropagateContext = c.Bool("marshal")
	} else {
		tracesCfg.TotalDuration = time.Duration(c.Int("duration") * int(time.Second))
		tracesCfg.Rate = c.Int64("rate")
		tracesCfg.NumTraces = c.Int("number-traces")
		tracesCfg.WorkerCount = c.Int("workers")
		tracesCfg.ExporterShards = c.Int("exporter-shards")
		tracesCfg.Scenarios = c.StringSlice("scenarios")
		tracesCfg.PropagateContext = c.Bool("marshal")
	}
	if tracesCfg.ExporterShards <= 0 {
		return fmt.Errorf("'exporter-shards' must be greater than 0")
	}

	if c.String("log-level") == "debug" {
		grpcZap.ReplaceGrpcLoggerV2(logger.WithOptions(
			zap.AddCallerSkip(3),
		))
	}

	grpcExpOpt := []otlptracegrpc.Option{
		otlptracegrpc.WithDialOption(
			grpc.WithBlock(),
		),
	}
	if tracesCfg.EndpointURL != "" {
		grpcExpOpt = append(grpcExpOpt, otlptracegrpc.WithEndpointURL(tracesCfg.EndpointURL))
	} else {
		grpcExpOpt = append(grpcExpOpt, otlptracegrpc.WithEndpoint(tracesCfg.Endpoint))
	}

	httpExpOpt := []otlptracehttp.Option{}
	if tracesCfg.EndpointURL != "" {
		httpExpOpt = append(httpExpOpt, otlptracehttp.WithEndpointURL(tracesCfg.EndpointURL))
	} else {
		httpExpOpt = append(httpExpOpt, otlptracehttp.WithEndpoint(tracesCfg.Endpoint))
	}
	if tracesCfg.URLPath != "" {
		httpExpOpt = append(httpExpOpt, otlptracehttp.WithURLPath(tracesCfg.URLPath))
	}

	if tracesCfg.Insecure {
		grpcExpOpt = append(grpcExpOpt, otlptracegrpc.WithInsecure())
		httpExpOpt = append(httpExpOpt, otlptracehttp.WithInsecure())
	}

	if len(c.StringSlice("header")) > 0 {
		headers := make(map[string]string)
		for _, h := range c.StringSlice("header") {
			kv := strings.SplitN(h, "=", 2)
			if len(kv) != 2 {
				return fmt.Errorf("value should be of the format key=value")
			}
			headers[kv[0]] = kv[1]
		}
		grpcExpOpt = append(grpcExpOpt, otlptracegrpc.WithHeaders(headers))
		httpExpOpt = append(httpExpOpt, otlptracehttp.WithHeaders(headers))
		tracesCfg.Headers = headers
	}
	httpExpOpt = append(httpExpOpt, otlptracehttp.WithCompression(otlptracehttp.GzipCompression))

	tracerProviders := make([]trace.TracerProvider, 0, tracesCfg.ExporterShards)
	for i := 0; i < tracesCfg.ExporterShards; i++ {
		var (
			exp *otlptrace.Exporter
			err error
		)
		if tracesCfg.UseHTTP {
			logger.Info("starting HTTP exporter", zap.Int("shard", i))
			exp, err = otlptracehttp.New(context.Background(), httpExpOpt...)
		} else {
			logger.Info("starting gRPC exporter", zap.Int("shard", i))
			exp, err = otlptracegrpc.New(context.Background(), grpcExpOpt...)
		}
		if err != nil {
			logger.Error("failed to obtain OTLP exporter", zap.Int("shard", i), zap.Error(err))
			return err
		}

		provider := sdktrace.NewTracerProvider(
			sdktrace.WithResource(resource.NewWithAttributes(semconv.SchemaURL, semconv.ServiceNameKey.String(tracesCfg.ServiceName))),
			sdktrace.WithBatcher(
				exp,
				sdktrace.WithBatchTimeout(100*time.Millisecond),
				sdktrace.WithMaxExportBatchSize(4096),
				sdktrace.WithMaxQueueSize(65536),
			),
		)
		defer func(provider *sdktrace.TracerProvider, shard int) {
			logger.Info("stopping trace provider", zap.Int("shard", shard))
			if err := provider.Shutdown(context.Background()); err != nil {
				logger.Error("failed to stop the trace provider", zap.Int("shard", shard), zap.Error(err))
			}
		}(provider, i)
		tracerProviders = append(tracerProviders, provider)
	}
	tracesCfg.TracerProviders = tracerProviders

	if err := traces.Run(tracesCfg, logger); err != nil {
		logger.Error("failed to run traces", zap.Error(err))
	}

	return nil
}
