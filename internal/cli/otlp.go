package cli

import (
	"errors"
	"time"

	"github.com/krzko/otelgen/internal/logs"
	"github.com/krzko/otelgen/internal/metrics"
	"github.com/krzko/otelgen/internal/traces"
	"github.com/urfave/cli/v2"
)

func validateOTLPEndpointFlags(c *cli.Context) error {
	if c.String("otel-exporter-otlp-endpoint") == "" && c.String("otel-exporter-otlp-endpoint-url") == "" {
		return errors.New("either 'otel-exporter-otlp-endpoint' or 'otel-exporter-otlp-endpoint-url' must be set")
	}
	return nil
}

func newMetricsConfig(c *cli.Context) *metrics.Config {
	return &metrics.Config{
		TotalDuration: time.Duration(c.Int("duration") * int(time.Second)),
		Endpoint:      c.String("otel-exporter-otlp-endpoint"),
		EndpointURL:   c.String("otel-exporter-otlp-endpoint-url"),
		URLPath:       c.String("otel-exporter-otlp-url-path"),
		Rate:          c.Int64("rate"),
		ServiceName:   c.String("service-name"),
	}
}

func newTracesConfig(c *cli.Context) *traces.Config {
	return &traces.Config{
		Endpoint:    c.String("otel-exporter-otlp-endpoint"),
		EndpointURL: c.String("otel-exporter-otlp-endpoint-url"),
		URLPath:     c.String("otel-exporter-otlp-url-path"),
		ServiceName: c.String("service-name"),
		Insecure:    c.Bool("insecure"),
		UseHTTP:     c.String("protocol") == "http",
	}
}

func newLogsConfig(c *cli.Context) *logs.Config {
	return &logs.Config{
		Endpoint:    c.String("otel-exporter-otlp-endpoint"),
		EndpointURL: c.String("otel-exporter-otlp-endpoint-url"),
		URLPath:     c.String("otel-exporter-otlp-url-path"),
		ServiceName: c.String("service-name"),
		Insecure:    c.Bool("insecure"),
		UseHTTP:     c.String("protocol") == "http",
	}
}
