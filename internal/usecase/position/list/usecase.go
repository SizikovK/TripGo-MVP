package list

import (
	"context"

	"github.com/google/uuid"

	"github.com/SizikovK/TripGo-MVP/internal/model"
)

type Usecase struct {
	repository repository
}

func New(repository repository) *Usecase {
	return &Usecase{repository: repository}
}

func (u *Usecase) Execute(ctx context.Context, id uuid.UUID) ([]model.Position, error) {
	return u.repository.ListPositions(ctx, id)
}
