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
