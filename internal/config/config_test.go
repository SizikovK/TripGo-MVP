package config

import "testing"

const collectorEndpoint = "OTEL_EXPORTER_OTLP_ENDPOINT"

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
	} {
		t.Run(tc.name, func(t *testing.T) {
			env := map[string]string{"DATABASE_URL": "postgres://localhost/tripgo", collectorEndpoint: "http://localhost:4317"}
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
