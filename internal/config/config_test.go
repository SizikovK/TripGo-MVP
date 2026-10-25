package config

import "testing"

func TestConfigurationValidation(t *testing.T) {
	for _, tc := range []struct {
		name, key, value string
		invalid          bool
	}{
		{"defaults", "", "", false},
		{"missing database", "DATABASE_URL", "", true},
		{"zero timeout", "DATABASE_QUERY_TIMEOUT", "0s", true},
		{"negative timeout", "SHUTDOWN_TIMEOUT", "-1s", true},
		{"invalid duration", "HTTP_READ_TIMEOUT", "abc", true},
		{"zero maximum", "DATABASE_MAX_CONNS", "0", true},
		{"negative minimum", "DATABASE_MIN_CONNS", "-1", true},
		{"minimum exceeds maximum", "DATABASE_MIN_CONNS", "26", true},
		{"overflow", "DATABASE_MAX_CONNS", "2147483648", true},
		{"invalid port", "HTTP_ADDR", ":99999", true},
		{"invalid level", "LOG_LEVEL", "verbose", true},
		{"zero minimum", "DATABASE_MIN_CONNS", "0", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			env := map[string]string{"DATABASE_URL": "postgres://localhost/tripgo"}
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
