//go:build integration

package trip_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	domain "github.com/SizikovK/TripGo-MVP/internal/domain/trip"
	"github.com/SizikovK/TripGo-MVP/internal/infra/postgres/transaction"
	"github.com/SizikovK/TripGo-MVP/internal/model"
	"github.com/SizikovK/TripGo-MVP/internal/repository/trip"
)

func TestPositions(t *testing.T) {
	ctx, pool := testPool(t)
	repository := trip.New(pool)
	tx := transaction.New(pool, time.Second)
	record := model.Trip{ID: uuid.New(), Data: domain.Data{UserID: uuid.New(), DriverID: uuid.New()}, Status: domain.StatusActive, StartedAt: time.Now().Add(-time.Hour)}
	require.NoError(t, repository.Create(ctx, record))

	fresh := time.Now().UTC().Truncate(time.Microsecond)
	for _, at := range []time.Time{fresh, fresh.Add(-time.Minute), fresh} {
		require.NoError(t, tx.Do(ctx, func(ctx context.Context) error {
			_, err := repository.SavePosition(ctx, record.ID, model.PositionData{RecordedAt: at})
			return err
		}))
	}

	actual, err := repository.Get(ctx, record.ID)
	require.NoError(t, err)
	require.NotNil(t, actual.LastPositionAt)
	require.True(t, fresh.Equal(*actual.LastPositionAt))

	positions, err := repository.ListPositions(ctx, record.ID)
	require.NoError(t, err)
	require.Len(t, positions, 3)
	require.True(t, positions[0].RecordedAt.Before(positions[1].RecordedAt))
	require.True(t, positions[1].RecordedAt.Equal(positions[2].RecordedAt))
	require.Less(t, positions[1].ID, positions[2].ID)

	err = tx.Do(ctx, func(ctx context.Context) error {
		_, err := repository.SavePosition(ctx, record.ID, model.PositionData{RecordedAt: fresh.Add(time.Minute), Coordinates: domain.Coordinates{Latitude: 91}})
		return err
	})
	require.Error(t, err)
	actual, err = repository.Get(ctx, record.ID)
	require.NoError(t, err)
	require.True(t, fresh.Equal(*actual.LastPositionAt))
	positions, err = repository.ListPositions(ctx, record.ID)
	require.NoError(t, err)
	require.Len(t, positions, 3)

	actual.Status = domain.StatusCompleted
	finished := fresh
	actual.FinishedAt = &finished
	require.NoError(t, repository.Finish(ctx, actual))
	err = tx.Do(ctx, func(ctx context.Context) error {
		_, err := repository.SavePosition(ctx, record.ID, model.PositionData{RecordedAt: fresh})
		return err
	})
	require.ErrorIs(t, err, domain.ErrTripCompleted)
	_, err = repository.ListPositions(ctx, uuid.New())
	require.ErrorIs(t, err, domain.ErrTripNotFound)
}

func TestStaleTrips(t *testing.T) {
	ctx, pool := testPool(t)
	repository := trip.New(pool)
	now := time.Now().UTC().Truncate(time.Microsecond)
	var expected []uuid.UUID
	for _, tc := range []struct {
		started   time.Time
		last      *time.Time
		completed bool
		stale     bool
	}{
		{started: now.Add(-3 * time.Hour), stale: true},
		{started: now.Add(-2 * time.Hour), last: timePointer(now.Add(-time.Hour)), stale: true},
		{started: now.Add(-time.Hour), last: &now},
		{started: now},
		{started: now.Add(-time.Hour), completed: true},
	} {
		record := model.Trip{ID: uuid.New(), Data: domain.Data{UserID: uuid.New(), DriverID: uuid.New()}, Status: domain.StatusActive, StartedAt: tc.started}
		if tc.completed {
			record.Status = domain.StatusCompleted
			record.FinishedAt = &now
		}

		require.NoError(t, repository.Create(ctx, record))
		_, err := pool.Exec(ctx, "UPDATE trips SET last_position_at=$2 WHERE id=$1", record.ID, tc.last)
		require.NoError(t, err)
		if tc.stale {
			expected = append(expected, record.ID)
		}
	}

	requests, err := repository.StaleTrips(ctx, time.Minute)
	require.NoError(t, err)
	require.Len(t, requests, len(expected))
	for i, request := range requests {
		require.Equal(t, expected[i], request.TripID)
		require.NotEqual(t, uuid.Nil, request.DriverID)
	}
}

func timePointer(at time.Time) *time.Time { return &at }
