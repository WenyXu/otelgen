package traces

import (
	"flag"
	"fmt"
	"strings"
	"time"

	"go.opentelemetry.io/otel/trace"
)

type Config struct {
	WorkerCount      int
	ExporterShards   int
	NumTraces        int
	PropagateContext bool
	Rate             int64
	TotalDuration    time.Duration
	ServiceName      string
	Scenarios        []string
	TracerProviders  []trace.TracerProvider

	// OTLP config
	Endpoint    string
	EndpointURL string
	URLPath     string
	Insecure    bool
	UseHTTP     bool
	Headers     HeaderValue
}

type HeaderValue map[string]string

var _ flag.Value = (*HeaderValue)(nil)

func (v *HeaderValue) String() string {
	return ""
}

func (v *HeaderValue) Set(s string) error {
	kv := strings.SplitN(s, "=", 2)
	if len(kv) != 2 {
		return fmt.Errorf("value should be of the format key=value")
	}
	(*v)[kv[0]] = kv[1]
	return nil
}
