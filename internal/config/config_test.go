package config

import "testing"

const (
	collectorEndpoint = "OTEL_EXPORTER_OTLP_ENDPOINT"
	pushEndpoint      = "PUSH_HTTP_URL"
)

func TestFromEnv(t *testing.T) {
	for _, tc := range []struct {
		name, key, value string
		invalid          bool
	}{
		{name: "defaults"},
		{name: "missing database", key: "DATABASE_URL", invalid: true},
		{name: "missing collector", key: collectorEndpoint, invalid: true},
		{name: "bad collector", key: collectorEndpoint, value: "not-a-url", invalid: true},
		{name: "bad protocol", key: "OTEL_EXPORTER_OTLP_PROTOCOL", value: "json", invalid: true},
		{name: "zero interval", key: "OTEL_METRIC_EXPORT_INTERVAL", value: "0", invalid: true},
		{name: "zero timeout", key: "DATABASE_QUERY_TIMEOUT", value: "0s", invalid: true},
		{name: "invalid duration", key: "SHUTDOWN_TIMEOUT", value: "none", invalid: true},
		{name: "negative connections", key: "DATABASE_MIN_CONNS", value: "-1", invalid: true},
		{name: "too many minimum connections", key: "DATABASE_MIN_CONNS", value: "26", invalid: true},
		{name: "overflow", key: "DATABASE_MAX_CONNS", value: "2147483648", invalid: true},
		{name: "bad port", key: "HTTP_ADDR", value: ":99999", invalid: true},
		{name: "bad level", key: "LOG_LEVEL", value: "verbose", invalid: true},
		{name: "missing push", key: pushEndpoint, invalid: true},
		{name: "bad push", key: pushEndpoint, value: "invalid", invalid: true},
		{name: "zero workers", key: "POSITION_WORKERS", value: "0", invalid: true},
		{name: "negative queue", key: "POSITION_QUEUE_SIZE", value: "-1", invalid: true},
		{name: "bad check interval", key: "POSITION_CHECK_INTERVAL", value: "0s", invalid: true},
		{name: "bad stale interval", key: "POSITION_STALE_AFTER", value: "invalid", invalid: true},
		{name: "bad push timeout", key: "PUSH_REQUEST_TIMEOUT", value: "-1s", invalid: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			env := map[string]string{"DATABASE_URL": "postgres://localhost/tripgo", collectorEndpoint: "http://localhost:4317", pushEndpoint: "http://localhost:8090"}
			if tc.key != "" {
				env[tc.key] = tc.value
			}
			_, err := fromEnv(func(key string) string { return env[key] })
			if (err != nil) != tc.invalid {
				t.Fatalf("invalid=%v, error=%v", tc.invalid, err)
			}
		})
	}
}
