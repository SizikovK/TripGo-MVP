package trip

import (
	"context"
	"fmt"
	"time"

	"github.com/SizikovK/TripGo-MVP/internal/model"
)

func (r *Repository) StaleTrips(ctx context.Context, staleAfter time.Duration) ([]model.PositionRequest, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, driver_id FROM trips
		WHERE status = 'active'
		  AND COALESCE(last_position_at, started_at) < now() - $1 * interval '1 second'
		ORDER BY COALESCE(last_position_at, started_at), id`, staleAfter.Seconds())
	if err != nil {
		return nil, fmt.Errorf("select stale trips: %w", err)
	}
	defer rows.Close()

	requests := make([]model.PositionRequest, 0)
	for rows.Next() {
		var request model.PositionRequest
		if err := rows.Scan(&request.TripID, &request.DriverID); err != nil {
			return nil, fmt.Errorf("scan stale trip: %w", err)
		}

		requests = append(requests, request)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read stale trips: %w", err)
	}

	return requests, nil
}
