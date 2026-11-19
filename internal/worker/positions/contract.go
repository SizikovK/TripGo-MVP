//go:generate go tool mockgen -source contract.go -destination mocks_test.go -package positions_test

package positions

import (
	"context"
	"time"

	"github.com/SizikovK/TripGo-MVP/internal/model"
)

type repository interface {
	StaleTrips(context.Context, time.Duration) ([]model.PositionRequest, error)
}

type pushGateway interface {
	RequestPosition(context.Context, model.PositionRequest) error
}

type ticker interface {
	Ticks() <-chan time.Time
	Stop()
}

type metrics interface {
	Queued(context.Context, string)
	QueueChanged(int64)
	ActiveChanged(int64)
	Processed(context.Context, string, time.Duration)
}
