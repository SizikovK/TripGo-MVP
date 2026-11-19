//go:generate go tool mockgen -source contract.go -destination mocks_test.go -package create_test

package create

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/SizikovK/TripGo-MVP/internal/model"
)

type repository interface {
	SavePosition(context.Context, uuid.UUID, model.PositionData) (model.Position, error)
}

type transactions interface {
	Do(context.Context, func(context.Context) error) error
}

type queue interface {
	Cancel(uuid.UUID)
}

type clock interface {
	Now() time.Time
}

type metrics interface {
	Saved(context.Context)
}
