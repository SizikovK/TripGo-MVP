//go:generate go tool mockgen -source contract.go -destination mocks_test.go -package finish_test

package finish

import (
	"context"

	"github.com/SizikovK/TripGo-MVP/internal/model"
	"github.com/google/uuid"
)

type tripRepository interface {
	Get(context.Context, uuid.UUID) (model.Trip, error)
	Finish(context.Context, model.Trip) error
	AddHistory(context.Context, uuid.UUID, *string, string) error
}

type txManager interface {
	Do(context.Context, func(context.Context) error) error
}

type metrics interface {
	Completed(context.Context)
}
