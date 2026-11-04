//go:build integration

package trip_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/joho/godotenv"
	"github.com/pressly/goose/v3"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/metric/noop"

	domain "github.com/SizikovK/TripGo-MVP/internal/domain/trip"
	"github.com/SizikovK/TripGo-MVP/internal/infra/postgres/transaction"
	"github.com/SizikovK/TripGo-MVP/internal/infra/telemetry"
	"github.com/SizikovK/TripGo-MVP/internal/model"
	"github.com/SizikovK/TripGo-MVP/internal/repository/trip"
	"github.com/SizikovK/TripGo-MVP/internal/usecase/trip/create"
	"github.com/SizikovK/TripGo-MVP/internal/usecase/trip/finish"
)

func testPool(t *testing.T) (context.Context, *pgxpool.Pool) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	t.Cleanup(cancel)
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		url = os.Getenv("DATABASE_URL")
	}
	if url == "" {
		env, err := godotenv.Read("../../../.env")
		require.NoError(t, err)
		url = env["DATABASE_URL"]
	}
	admin, err := pgx.Connect(ctx, url)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, admin.Close(ctx)) })
	schema := "tripgo_test_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	quoted := pgx.Identifier{schema}.Sanitize()
	_, err = admin.Exec(ctx, "CREATE SCHEMA "+quoted)
	require.NoError(t, err)
	t.Cleanup(func() {
		cleanup, stop := context.WithTimeout(context.Background(), 5*time.Second)
		defer stop()
		_, err := admin.Exec(cleanup, "DROP SCHEMA "+quoted+" CASCADE")
		require.NoError(t, err)
	})
	cfg, err := pgxpool.ParseConfig(url)
	require.NoError(t, err)
	cfg.ConnConfig.RuntimeParams["search_path"] = schema
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	database := stdlib.OpenDB(*cfg.ConnConfig)
	t.Cleanup(func() { require.NoError(t, database.Close()) })
	provider, err := goose.NewProvider(goose.DialectPostgres, database, os.DirFS("../../../migrations"))
	require.NoError(t, err)
	_, err = provider.Up(ctx)
	require.NoError(t, err)
	_, err = provider.DownTo(ctx, 0)
	require.NoError(t, err)
	_, err = provider.Up(ctx)
	require.NoError(t, err)
	return ctx, pool
}

func TestTransactionsAndConcurrency(t *testing.T) {
	ctx, pool := testPool(t)
	repository := trip.New(pool)
	transactions := transaction.New(pool, time.Second)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	metrics, err := telemetry.NewTrips(noop.NewMeterProvider().Meter("test"), repository.CountActive, time.Second, logger)
	require.NoError(t, err)
	creator := create.New(repository, transactions, logger, metrics)
	finisher := finish.New(repository, transactions, logger, metrics)
	data := model.CreateTrip{UserID: uuid.New(), DriverID: uuid.New(), Price: 100}
	record := model.Trip{Data: data, ID: uuid.New(), Status: domain.StatusActive, StartedAt: time.Now().UTC()}
	failure := errors.New("test rollback")
	err = transactions.Do(ctx, func(ctx context.Context) error {
		return transactions.Do(ctx, func(ctx context.Context) error {
			if err := repository.Create(ctx, record); err != nil {
				return err
			}
			return failure
		})
	})
	require.ErrorIs(t, err, failure)
	_, err = repository.Get(ctx, record.ID)
	require.ErrorIs(t, err, domain.ErrTripNotFound)
	err = transactions.Do(ctx, func(ctx context.Context) error {
		if err := repository.Create(ctx, record); err != nil {
			return err
		}
		return repository.AddHistory(ctx, record.ID, nil, "invalid")
	})
	require.Error(t, err)
	_, err = repository.Get(ctx, record.ID)
	require.ErrorIs(t, err, domain.ErrTripNotFound)
	require.PanicsWithValue(t, "test panic", func() {
		_ = transactions.Do(ctx, func(ctx context.Context) error {
			require.NoError(t, repository.Create(ctx, record))
			panic("test panic")
		})
	})
	_, err = repository.Get(ctx, record.ID)
	require.ErrorIs(t, err, domain.ErrTripNotFound)
	var group sync.WaitGroup
	results := make(chan error, 20)
	created := make(chan model.Trip, 20)
	for range 20 {
		group.Go(func() {
			result, err := creator.Execute(ctx, data)
			if err == nil {
				created <- result
			}
			results <- err
		})
	}
	group.Wait()
	close(results)
	successes := 0
	for err := range results {
		if err == nil {
			successes++
		} else {
			require.ErrorIs(t, err, domain.ErrDriverBusy)
		}
	}
	require.Equal(t, 1, successes)
	record = <-created
	results = make(chan error, 2)
	for range 2 {
		group.Go(func() { _, err := finisher.Execute(ctx, record.ID); results <- err })
	}
	group.Wait()
	close(results)
	successes = 0
	for err := range results {
		if err == nil {
			successes++
		} else {
			require.ErrorIs(t, err, domain.ErrTripCompleted)
		}
	}
	require.Equal(t, 1, successes)
	actual, err := repository.Get(ctx, record.ID)
	require.NoError(t, err)
	require.Equal(t, domain.StatusCompleted, actual.Status)
	require.NotNil(t, actual.FinishedAt)
	var history int
	require.NoError(t, pool.QueryRow(ctx, "SELECT count(*) FROM trip_status_history WHERE trip_id=$1", record.ID).Scan(&history))
	require.Equal(t, 2, history)
	count, err := repository.CountActive(ctx)
	require.NoError(t, err)
	require.Zero(t, count)
}
