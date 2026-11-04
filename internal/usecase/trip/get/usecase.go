package get

import (
	"context"

	"github.com/google/uuid"

	"github.com/SizikovK/TripGo-MVP/internal/model"
)

type Usecase struct{ repository tripRepository }

func New(repository tripRepository) *Usecase { return &Usecase{repository: repository} }

func (u *Usecase) Execute(ctx context.Context, id uuid.UUID) (model.Trip, error) {
	return u.repository.Get(ctx, id)
}
