//go:generate go tool mockgen -source contract.go -destination mocks_test.go -package get_test

package get

import (
	"context"

	"github.com/SizikovK/TripGo-MVP/internal/model"
	"github.com/google/uuid"
)

type tripRepository interface {
	Get(context.Context, uuid.UUID) (model.Trip, error)
}
