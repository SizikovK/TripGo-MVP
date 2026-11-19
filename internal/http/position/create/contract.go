//go:generate go tool mockgen -source contract.go -destination mocks_test.go -package create_test

package create

import (
	"context"

	"github.com/google/uuid"

	"github.com/SizikovK/TripGo-MVP/internal/model"
)

type usecase interface {
	Execute(context.Context, uuid.UUID, model.PositionData) (model.Position, error)
}
