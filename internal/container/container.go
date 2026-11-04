package container

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/SizikovK/TripGo-MVP/internal/config"
	httpserver "github.com/SizikovK/TripGo-MVP/internal/http"
	httpcreate "github.com/SizikovK/TripGo-MVP/internal/http/trip/create"
	httpfinish "github.com/SizikovK/TripGo-MVP/internal/http/trip/finish"
	httpget "github.com/SizikovK/TripGo-MVP/internal/http/trip/get"
	"github.com/SizikovK/TripGo-MVP/internal/infra/postgres"
	"github.com/SizikovK/TripGo-MVP/internal/infra/postgres/transaction"
	"github.com/SizikovK/TripGo-MVP/internal/infra/telemetry"
	"github.com/SizikovK/TripGo-MVP/internal/repository/trip"
	"github.com/SizikovK/TripGo-MVP/internal/usecase/trip/create"
	"github.com/SizikovK/TripGo-MVP/internal/usecase/trip/finish"
	"github.com/SizikovK/TripGo-MVP/internal/usecase/trip/get"
)

type Container struct {
	Server          *http.Server
	Logger          *slog.Logger
	pool            *pgxpool.Pool
	telemetry       *telemetry.Providers
	shutdownTimeout time.Duration
}

func New(ctx context.Context, cfg *config.Config) (*Container, error) {
	logger, err := telemetry.Logger(cfg.LogLevel, cfg.ServiceName)
	if err != nil {
		return nil, err
	}
	providers, err := telemetry.New(ctx, cfg, logger)
	if err != nil {
		return nil, err
	}
	app := &Container{Logger: logger, telemetry: providers, shutdownTimeout: cfg.ShutdownTimeout}
	pool, err := postgres.New(ctx, cfg, providers.Traces, providers.Meters)
	if err != nil {
		_ = app.Close(ctx)
		return nil, err
	}
	app.pool = pool
	repository := trip.New(pool)
	transactions := transaction.New(pool, cfg.DatabaseQueryTimeout)
	metrics, err := telemetry.NewTrips(providers.Meters.Meter("tripgo.trips"), repository.CountActive, cfg.DatabaseQueryTimeout, logger)
	if err != nil {
		_ = app.Close(ctx)
		return nil, err
	}
	createTrip := create.New(repository, transactions, logger, metrics)
	getTrip := get.New(repository)
	finishTrip := finish.New(repository, transactions, logger, metrics)
	server, err := httpserver.NewServer(cfg, httpserver.Dependencies{
		Create:    httpcreate.New(createTrip, logger, cfg.DatabaseQueryTimeout),
		Get:       httpget.New(getTrip, logger, cfg.DatabaseQueryTimeout),
		Finish:    httpfinish.New(finishTrip, logger, cfg.DatabaseQueryTimeout),
		Readiness: pool, Logger: logger, Traces: providers.Traces, Meters: providers.Meters,
	})
	if err != nil {
		_ = app.Close(ctx)
		return nil, err
	}
	app.Server = server
	return app, nil
}

func (c *Container) Close(ctx context.Context) error {
	cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), c.shutdownTimeout)
	defer cancel()
	var err error
	if c.Server != nil {
		err = c.Server.Close()
	}
	err = errors.Join(err, c.telemetry.Shutdown(cleanup))
	if c.pool != nil {
		c.pool.Close()
	}
	if err != nil {
		return fmt.Errorf("close dependencies: %w", err)
	}
	return nil
}
