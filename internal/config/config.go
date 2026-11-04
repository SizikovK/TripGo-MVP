package config

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	ServiceName             string
	ServiceVersion          string
	OTelEndpoint            string
	OTelProtocol            string
	OTelSampler             string
	OTelSamplerArg          string
	OTelMetricInterval      time.Duration
	HttpAddr                string
	LogLevel                string
	ShutdownTimeout         time.Duration
	DatabaseUrl             string
	DatabaseMaxConns        int32
	DatabaseMinConns        int32
	DatabaseConnectTimeout  time.Duration
	DatabaseQueryTimeout    time.Duration
	DatabaseMaxConnLifetime time.Duration
	ServerReadTimeout       time.Duration
	ServerReadHeaderTimeout time.Duration
	ServerWriteTimeout      time.Duration
	ServerIdleTimeout       time.Duration
}

func New() (*Config, error) {
	if err := godotenv.Load(); err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, errors.New("не удалось прочитать .env")
	}
	return fromEnv(os.Getenv)
}

func fromEnv(getenv func(string) string) (*Config, error) {
	value := func(key, fallback string) string {
		if v := getenv(key); v != "" {
			return v
		}
		return fallback
	}
	cfg := &Config{
		ServiceName:    value("OTEL_SERVICE_NAME", "trip-service"),
		ServiceVersion: value("SERVICE_VERSION", "local"),
		OTelEndpoint:   getenv("OTEL_EXPORTER_OTLP_ENDPOINT"),
		OTelProtocol:   value("OTEL_EXPORTER_OTLP_PROTOCOL", "grpc"),
		OTelSampler:    value("OTEL_TRACES_SAMPLER", "parentbased_always_on"),
		OTelSamplerArg: value("OTEL_TRACES_SAMPLER_ARG", "1"),
		HttpAddr:       value("HTTP_ADDR", ":8080"),
		LogLevel:       strings.ToLower(value("LOG_LEVEL", "info")),
		DatabaseUrl:    getenv("DATABASE_URL"),
	}
	if err := cfg.validateAddress(); err != nil {
		return nil, err
	}
	var err error
	interval, err := getInt("OTEL_METRIC_EXPORT_INTERVAL", value("OTEL_METRIC_EXPORT_INTERVAL", "5000"))
	if err != nil || interval <= 0 {
		return nil, errors.New("OTEL_METRIC_EXPORT_INTERVAL должен быть положительным числом миллисекунд")
	}
	cfg.OTelMetricInterval = time.Duration(interval) * time.Millisecond
	if cfg.OTelEndpoint == "" {
		return nil, errors.New("OTEL_EXPORTER_OTLP_ENDPOINT обязателен")
	}
	endpoint, err := url.Parse(cfg.OTelEndpoint)
	if err != nil || endpoint.Host == "" || (endpoint.Scheme != "http" && endpoint.Scheme != "https") {
		return nil, errors.New("OTEL_EXPORTER_OTLP_ENDPOINT должен быть HTTP или HTTPS URL")
	}
	if cfg.OTelProtocol != "grpc" && cfg.OTelProtocol != "http/protobuf" {
		return nil, errors.New("OTEL_EXPORTER_OTLP_PROTOCOL должен быть grpc или http/protobuf")
	}
	cfg.ShutdownTimeout, err = getDuration("SHUTDOWN_TIMEOUT", value("SHUTDOWN_TIMEOUT", "30s"))
	if err != nil {
		return nil, err
	}
	cfg.DatabaseConnectTimeout, err = getDuration("DATABASE_CONNECT_TIMEOUT", value("DATABASE_CONNECT_TIMEOUT", "5s"))
	if err != nil {
		return nil, err
	}
	cfg.DatabaseQueryTimeout, err = getDuration("DATABASE_QUERY_TIMEOUT", value("DATABASE_QUERY_TIMEOUT", "3s"))
	if err != nil {
		return nil, err
	}
	cfg.DatabaseMaxConnLifetime, err = getDuration("DATABASE_MAX_CONN_LIFETIME", value("DATABASE_MAX_CONN_LIFETIME", "30m"))
	if err != nil {
		return nil, err
	}
	cfg.ServerReadTimeout, err = getDuration("HTTP_READ_TIMEOUT", value("HTTP_READ_TIMEOUT", "10s"))
	if err != nil {
		return nil, err
	}
	cfg.ServerReadHeaderTimeout, err = getDuration("HTTP_READ_HEADER_TIMEOUT", value("HTTP_READ_HEADER_TIMEOUT", "5s"))
	if err != nil {
		return nil, err
	}
	cfg.ServerWriteTimeout, err = getDuration("HTTP_WRITE_TIMEOUT", value("HTTP_WRITE_TIMEOUT", "15s"))
	if err != nil {
		return nil, err
	}
	cfg.ServerIdleTimeout, err = getDuration("HTTP_IDLE_TIMEOUT", value("HTTP_IDLE_TIMEOUT", "60s"))
	if err != nil {
		return nil, err
	}
	cfg.DatabaseMaxConns, err = getInt("DATABASE_MAX_CONNS", value("DATABASE_MAX_CONNS", "25"))
	if err != nil {
		return nil, err
	}
	cfg.DatabaseMinConns, err = getInt("DATABASE_MIN_CONNS", value("DATABASE_MIN_CONNS", "5"))
	if err != nil {
		return nil, err
	}
	if cfg.DatabaseMaxConns <= 0 || cfg.DatabaseMinConns < 0 || cfg.DatabaseMinConns > cfg.DatabaseMaxConns {
		return nil, errors.New("пул БД: требуется 0 <= DATABASE_MIN_CONNS <= DATABASE_MAX_CONNS и DATABASE_MAX_CONNS > 0")
	}
	return cfg, nil
}

func (cfg *Config) validateAddress() error {
	if strings.TrimSpace(cfg.DatabaseUrl) == "" {
		return errors.New("DATABASE_URL обязателен")
	}
	_, port, err := net.SplitHostPort(cfg.HttpAddr)
	if err != nil {
		return errors.New("HTTP_ADDR должен иметь вид host:port или :port")
	}
	p, err := strconv.Atoi(port)
	if err != nil || p < 1 || p > 65535 {
		return errors.New("порт HTTP_ADDR должен быть в диапазоне 1..65535")
	}
	switch cfg.LogLevel {
	case "debug", "info", "warn", "error":
	default:
		return errors.New("LOG_LEVEL должен быть debug, info, warn или error")
	}
	return nil
}

func getDuration(key, value string) (time.Duration, error) {
	duration, err := time.ParseDuration(value)
	if err != nil || duration <= 0 {
		return 0, fmt.Errorf("%s должен быть положительной длительностью, например 5s", key)
	}
	return duration, nil
}

func getInt(key, value string) (int32, error) {
	number, err := strconv.ParseInt(value, 10, 32)
	if err != nil {
		return 0, fmt.Errorf("%s должен быть целым числом в диапазоне int32", key)
	}
	return int32(number), nil
}
