package cli

import (
	"context"
	"time"

	"github.com/krzko/otelgen/internal/metrics"
	"github.com/urfave/cli/v2"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"go.uber.org/zap"
)

var generateMetricsHistogramCommand = &cli.Command{
	Name:        "histogram",
	Usage:       "generate metrics of type histogram",
	Description: "Histogram demonstrates how to measure a distribution of values",
	Aliases:     []string{"hist"},
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:  "temporality",
			Usage: "Temporality defines the window that an aggregation was calculated over, one of: delta, cumulative",
			Value: "cumulative",
		},
		&cli.StringFlag{
			Name:  "unit",
			Usage: "Unit of measurement for the histogram",
			Value: "ms",
		},
		&cli.StringSliceFlag{
			Name:  "attribute",
			Usage: "Attributes to add to the histogram (format: key=value)",
		},
		&cli.Float64SliceFlag{
			Name:  "bounds",
			Usage: "Bucket boundaries for the histogram",
			Value: cli.NewFloat64Slice(1, 5, 10, 25, 50, 100, 250, 500, 1000),
		},
		&cli.BoolFlag{
			Name:  "record-minmax",
			Usage: "Record min and max values",
			Value: true,
		},
	},
	Action: func(c *cli.Context) error {
		return generateMetricsHistogramAction(c)
	},
}

func generateMetricsHistogramAction(c *cli.Context) error {
	if err := validateOTLPEndpointFlags(c); err != nil {
		return err
	}

	metricsCfg := newMetricsConfig(c)

	configureLogging(c)

	grpcExpOpt, httpExpOpt := getExporterOptions(c, metricsCfg)

	ctx := context.Background()

	exp, err := createExporter(ctx, c, grpcExpOpt, httpExpOpt)
	if err != nil {
		logger.Error("failed to obtain OTLP exporter", zap.Error(err))
		return err
	}
	defer shutdownExporter(exp)

	logger.Info("Starting metrics generation")

	reader := metric.NewPeriodicReader(
		exp,
		metric.WithInterval(time.Duration(metricsCfg.Rate)*time.Second),
	)

	provider := createMeterProvider(reader, metricsCfg)

	temporality := metricdata.CumulativeTemporality
	if c.String("temporality") == "delta" {
		temporality = metricdata.DeltaTemporality
	}

	attributes, err := parseAttributes(c.StringSlice("attribute"))
	if err != nil {
		logger.Error("failed to parse attributes", zap.Error(err))
		return err
	}

	histogramConfig := metrics.HistogramConfig{
		Name:         metricsCfg.ServiceName + ".metrics.histogram",
		Description:  "Histogram demonstrates how to measure a distribution of values",
		Unit:         c.String("unit"),
		Attributes:   attributes,
		Temporality:  temporality,
		Bounds:       c.Float64Slice("bounds"),
		RecordMinMax: c.Bool("record-minmax"),
	}

	metrics.SimulateHistogram(provider, histogramConfig, metricsCfg, logger)

	return nil
}
