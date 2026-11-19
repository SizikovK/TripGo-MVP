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
	"github.com/SizikovK/TripGo-MVP/internal/gateway/push"
	httpserver "github.com/SizikovK/TripGo-MVP/internal/http"
	positionhttpcreate "github.com/SizikovK/TripGo-MVP/internal/http/position/create"
	positionhttplist "github.com/SizikovK/TripGo-MVP/internal/http/position/list"
	httpcreate "github.com/SizikovK/TripGo-MVP/internal/http/trip/create"
	httpfinish "github.com/SizikovK/TripGo-MVP/internal/http/trip/finish"
	httpget "github.com/SizikovK/TripGo-MVP/internal/http/trip/get"
	"github.com/SizikovK/TripGo-MVP/internal/infra/clock"
	"github.com/SizikovK/TripGo-MVP/internal/infra/postgres"
	"github.com/SizikovK/TripGo-MVP/internal/infra/postgres/transaction"
	"github.com/SizikovK/TripGo-MVP/internal/infra/telemetry"
	"github.com/SizikovK/TripGo-MVP/internal/repository/trip"
	positioncreate "github.com/SizikovK/TripGo-MVP/internal/usecase/position/create"
	positionlist "github.com/SizikovK/TripGo-MVP/internal/usecase/position/list"
	"github.com/SizikovK/TripGo-MVP/internal/usecase/trip/create"
	"github.com/SizikovK/TripGo-MVP/internal/usecase/trip/finish"
	"github.com/SizikovK/TripGo-MVP/internal/usecase/trip/get"
	"github.com/SizikovK/TripGo-MVP/internal/worker/positions"
)

type Container struct {
	Server          *http.Server
	Logger          *slog.Logger
	pool            *pgxpool.Pool
	telemetry       *telemetry.Providers
	shutdownTimeout time.Duration
	positions       *positions.Pool
	push            *push.Gateway
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
	positionMetrics, err := telemetry.NewPositions(providers.Meters.Meter("tripgo.positions"))
	if err != nil {
		_ = app.Close(ctx)
		return nil, err
	}

	app.push, err = push.New(cfg.PushHTTPURL, cfg.PushRequestTimeout, providers.Traces)
	if err != nil {
		_ = app.Close(ctx)
		return nil, err
	}

	app.positions = positions.New(cfg, repository, app.push, clock.NewTicker(cfg.PositionCheckInterval), positionMetrics, logger, providers.Traces.Tracer("tripgo.positions"))
	createPosition := positioncreate.New(repository, transactions, app.positions, clock.Clock{}, positionMetrics, logger)
	listPositions := positionlist.New(repository)
	server, err := httpserver.NewServer(cfg, httpserver.Dependencies{
		CreatePosition: positionhttpcreate.New(createPosition, logger, cfg.DatabaseQueryTimeout),
		ListPositions:  positionhttplist.New(listPositions, logger, cfg.DatabaseQueryTimeout),
		Create:         httpcreate.New(createTrip, logger, cfg.DatabaseQueryTimeout),
		Get:            httpget.New(getTrip, logger, cfg.DatabaseQueryTimeout),
		Finish:         httpfinish.New(finishTrip, logger, cfg.DatabaseQueryTimeout),
		Readiness:      pool, Logger: logger, Traces: providers.Traces, Meters: providers.Meters,
	})
	if err != nil {
		_ = app.Close(ctx)
		return nil, err
	}

	app.Server = server

	return app, nil
}

func (c *Container) Start(ctx context.Context) {
	c.positions.Start(ctx)
}

func (c *Container) Close(ctx context.Context) error {
	timeout := c.shutdownTimeout
	if deadline, ok := ctx.Deadline(); ok {
		timeout = min(timeout, time.Until(deadline))
	}

	cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), timeout)
	defer cancel()
	var err error
	if c.Server != nil {
		err = c.Server.Close()
	}

	if c.positions != nil {
		err = errors.Join(err, c.positions.Shutdown(cleanup))
	}
	if c.push != nil {
		c.push.Close()
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
