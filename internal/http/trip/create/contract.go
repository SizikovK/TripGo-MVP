//go:generate go tool mockgen -source contract.go -destination mocks_test.go -package create_test

package create

import (
	"context"

	"github.com/SizikovK/TripGo-MVP/internal/model"
)

type usecase interface {
	Execute(context.Context, model.CreateTrip) (model.Trip, error)
}
