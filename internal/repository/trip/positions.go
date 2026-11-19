package trip

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	domain "github.com/SizikovK/TripGo-MVP/internal/domain/trip"
	"github.com/SizikovK/TripGo-MVP/internal/infra/postgres/transaction"
	"github.com/SizikovK/TripGo-MVP/internal/model"
)

func (r *Repository) SavePosition(ctx context.Context, id uuid.UUID, data model.PositionData) (model.Position, error) {
	connection := transaction.Connection(ctx, r.pool)
	tag, err := connection.Exec(ctx, `
		UPDATE trips SET last_position_at = GREATEST(last_position_at, $2), updated_at = now()
		WHERE id = $1 AND status = 'active'`, id, data.RecordedAt)
	if err != nil {
		return model.Position{}, fmt.Errorf("update last position: %w", err)
	}

	if tag.RowsAffected() == 0 {
		if _, err := r.Get(ctx, id); err != nil {
			return model.Position{}, err
		}

		return model.Position{}, domain.ErrTripCompleted
	}

	position := model.Position{TripID: id, PositionData: data}
	err = connection.QueryRow(ctx, `
		INSERT INTO trip_positions (trip_id, latitude, longitude, recorded_at)
		VALUES ($1, $2, $3, $4) RETURNING id`, id, data.Coordinates.Latitude, data.Coordinates.Longitude, data.RecordedAt).Scan(&position.ID)
	if err != nil {
		return model.Position{}, fmt.Errorf("insert position: %w", err)
	}

	return position, nil
}

func (r *Repository) ListPositions(ctx context.Context, id uuid.UUID) ([]model.Position, error) {
	record, err := r.Get(ctx, id)
	if err != nil {
		return nil, err
	}

	rows, err := transaction.Connection(ctx, r.pool).Query(ctx, `
		SELECT id, trip_id, latitude, longitude, recorded_at
		FROM trip_positions WHERE trip_id = $1 ORDER BY recorded_at, id`, record.ID)
	if err != nil {
		return nil, fmt.Errorf("list positions: %w", err)
	}
	defer rows.Close()

	positions := make([]model.Position, 0)
	for rows.Next() {
		var position model.Position
		if err := rows.Scan(&position.ID, &position.TripID, &position.Coordinates.Latitude, &position.Coordinates.Longitude, &position.RecordedAt); err != nil {
			return nil, fmt.Errorf("scan position: %w", err)
		}

		positions = append(positions, position)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read positions: %w", err)
	}

	return positions, nil
}
