package trip

import (
	"context"
	"errors"
	"fmt"

	sq "github.com/Masterminds/squirrel"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	domain "github.com/SizikovK/TripGo-MVP/internal/domain/trip"
	"github.com/SizikovK/TripGo-MVP/internal/infra/postgres/transaction"
	"github.com/SizikovK/TripGo-MVP/internal/model"
)

const columns = "id, user_id, driver_id, start_latitude, start_longitude, end_latitude, end_longitude, price, status, started_at, finished_at"

type Repository struct {
	pool    database
	builder sq.StatementBuilderType
}

func New(pool database) *Repository {
	return &Repository{pool: pool, builder: sq.StatementBuilder.PlaceholderFormat(sq.Dollar)}
}

func (r *Repository) Get(ctx context.Context, id uuid.UUID) (model.Trip, error) {
	query, args, err := r.builder.Select(columns).From("trips").Where(sq.Eq{"id": id}).ToSql()
	if err != nil {
		return model.Trip{}, fmt.Errorf("build get trip: %w", err)
	}
	record, err := scanTrip(transaction.Connection(ctx, r.pool).QueryRow(ctx, query, args...))
	if errors.Is(err, pgx.ErrNoRows) {
		return model.Trip{}, domain.ErrTripNotFound
	}
	if err != nil {
		return model.Trip{}, fmt.Errorf("get trip: %w", err)
	}
	return record, nil
}

func (r *Repository) Create(ctx context.Context, record model.Trip) error {
	query, args, err := r.builder.Insert("trips").Columns("id", "user_id", "driver_id", "start_latitude", "start_longitude", "end_latitude", "end_longitude", "price", "status", "started_at", "finished_at").Values(
		record.ID, record.UserID, record.DriverID, record.StartPoint.Latitude, record.StartPoint.Longitude,
		record.EndPoint.Latitude, record.EndPoint.Longitude, record.Price, record.Status, record.StartedAt, record.FinishedAt,
	).ToSql()
	if err != nil {
		return fmt.Errorf("build create trip: %w", err)
	}
	_, err = transaction.Connection(ctx, r.pool).Exec(ctx, query, args...)
	var pgError *pgconn.PgError
	if errors.As(err, &pgError) && pgError.Code == "23505" && pgError.ConstraintName == "trips_one_active_per_driver_idx" {
		return domain.ErrDriverBusy
	}
	if err != nil {
		return fmt.Errorf("insert trip: %w", err)
	}
	return nil
}

func (r *Repository) Finish(ctx context.Context, record model.Trip) error {
	query, args, err := r.builder.Update("trips").Set("status", record.Status).Set("finished_at", record.FinishedAt).
		Set("updated_at", record.FinishedAt).Where(sq.Eq{"id": record.ID, "status": domain.StatusActive}).ToSql()
	if err != nil {
		return fmt.Errorf("build finish trip: %w", err)
	}
	tag, err := transaction.Connection(ctx, r.pool).Exec(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("update trip: %w", err)
	}
	if tag.RowsAffected() == 0 {
		if _, err := r.Get(ctx, record.ID); err != nil {
			return err
		}
		return domain.ErrTripCompleted
	}
	return nil
}

func (r *Repository) AddHistory(ctx context.Context, id uuid.UUID, from *string, to string) error {
	query, args, err := r.builder.Insert("trip_status_history").Columns("trip_id", "from_status", "to_status").Values(id, from, to).ToSql()
	if err != nil {
		return fmt.Errorf("build history: %w", err)
	}
	if _, err := transaction.Connection(ctx, r.pool).Exec(ctx, query, args...); err != nil {
		return fmt.Errorf("insert history: %w", err)
	}
	return nil
}

func (r *Repository) CountActive(ctx context.Context) (int64, error) {
	query, args, err := r.builder.Select("count(*)").From("trips").Where(sq.Eq{"status": domain.StatusActive}).ToSql()
	if err != nil {
		return 0, fmt.Errorf("build count: %w", err)
	}
	var count int64
	if err := r.pool.QueryRow(ctx, query, args...).Scan(&count); err != nil {
		return 0, fmt.Errorf("count active trips: %w", err)
	}
	return count, nil
}
