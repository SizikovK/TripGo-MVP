package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	sq "github.com/Masterminds/squirrel"
	api "github.com/SizikovK/TripGo-MVP/internal/generated"
	"github.com/SizikovK/TripGo-MVP/internal/trips"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var sqlBuilder = sq.StatementBuilder.PlaceholderFormat(sq.Dollar)

var tripColumns = []string{
	"id",
	"user_id",
	"driver_id",
	"start_latitude",
	"start_longitude",
	"end_latitude",
	"end_longitude",
	"price",
	"status",
	"started_at",
	"finished_at",
}

type TripRepository struct {
	pool *pgxpool.Pool
}

func NewTripRepository(pool *pgxpool.Pool) *TripRepository {
	return &TripRepository{pool: pool}
}

func scanTrip(row pgx.Row) (trip api.Trip, err error) {
	err = row.Scan(&trip.Id, &trip.UserId, &trip.DriverId,
		&trip.StartPoint.Latitude, &trip.StartPoint.Longitude,
		&trip.EndPoint.Latitude, &trip.EndPoint.Longitude,
		&trip.Price, &trip.Status, &trip.StartedAt, &trip.FinishedAt)
	return trip, err
}

func (r *TripRepository) Create(ctx context.Context, trip api.Trip) error {
	query, args, err := sqlBuilder.Insert("trips").Columns(tripColumns...).Values(
		trip.Id, trip.UserId, trip.DriverId, trip.StartPoint.Latitude, trip.StartPoint.Longitude,
		trip.EndPoint.Latitude, trip.EndPoint.Longitude, trip.Price, trip.Status, trip.StartedAt, trip.FinishedAt,
	).ToSql()

	if err != nil {
		return fmt.Errorf("SQL создания поездки: %w", err)
	}

	_, err = connection(ctx, r.pool).Exec(ctx, query, args...)
	var pgErr *pgconn.PgError

	if errors.As(err, &pgErr) && pgErr.Code == "23505" && pgErr.ConstraintName == "trips_one_active_per_driver_idx" {
		return trips.ErrDriverBusy
	}
	if err != nil {
		return fmt.Errorf("создание поездки: %w", err)
	}

	return nil
}

func (r *TripRepository) Get(ctx context.Context, id uuid.UUID) (api.Trip, error) {
	query, args, err := sqlBuilder.
		Select(tripColumns...).
		From("trips").
		Where(sq.Eq{"id": id}).
		ToSql()
	if err != nil {
		return api.Trip{}, fmt.Errorf("SQL получения поездки: %w", err)
	}
	trip, err := scanTrip(connection(ctx, r.pool).QueryRow(ctx, query, args...))

	if errors.Is(err, pgx.ErrNoRows) {
		return api.Trip{}, trips.ErrNotFound
	}

	if err != nil {
		return api.Trip{}, fmt.Errorf("получение поездки: %w", err)
	}

	return trip, nil
}

func (r *TripRepository) Finish(ctx context.Context, id uuid.UUID, now time.Time) (api.Trip, error) {
	query, args, err := sqlBuilder.
		Update("trips").
		Set("status", string(api.Completed)).
		Set("finished_at", now).
		Set("updated_at", now).
		Where(sq.Eq{"id": id, "status": string(api.Active)}).
		Suffix("RETURNING " + strings.Join(tripColumns, ", ")).
		ToSql()

	if err != nil {
		return api.Trip{}, fmt.Errorf("SQL завершения поездки: %w", err)
	}

	trip, err := scanTrip(connection(ctx, r.pool).QueryRow(ctx, query, args...))
	if errors.Is(err, pgx.ErrNoRows) {
		if _, getErr := r.Get(ctx, id); getErr != nil {
			return api.Trip{}, getErr
		}
		return api.Trip{}, trips.ErrCompleted
	}

	if err != nil {
		return api.Trip{}, fmt.Errorf("завершение поездки: %w", err)
	}
	return trip, nil
}

func (r *TripRepository) AddHistory(ctx context.Context, id uuid.UUID, from *string, to string) error {
	query, args, err := sqlBuilder.Insert("trip_status_history").
		Columns("trip_id", "from_status", "to_status").Values(id, from, to).ToSql()
	if err != nil {
		return fmt.Errorf("SQL истории статуса: %w", err)
	}
	if _, err := connection(ctx, r.pool).Exec(ctx, query, args...); err != nil {
		return fmt.Errorf("запись истории статуса: %w", err)
	}
	return nil
}
