//go:generate go tool mockgen -source contract.go -destination mocks_test.go -package list_test

package list

import (
	"context"

	"github.com/google/uuid"

	"github.com/SizikovK/TripGo-MVP/internal/model"
)

type repository interface {
	ListPositions(context.Context, uuid.UUID) ([]model.Position, error)
}
