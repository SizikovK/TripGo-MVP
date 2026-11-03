package create

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/SizikovK/TripGo-MVP/internal/domain/trip"
	"github.com/SizikovK/TripGo-MVP/internal/model"
	"github.com/google/uuid"
)

type Usecase struct {
	repository   tripRepository
	transactions txManager
	logger       *slog.Logger
	metrics      metrics
}

func New(repository tripRepository, transactions txManager, logger *slog.Logger, metrics metrics) *Usecase {
	return &Usecase{repository: repository, transactions: transactions, logger: logger, metrics: metrics}
}

func (u *Usecase) Execute(ctx context.Context, data model.CreateTrip) (model.Trip, error) {
	if err := data.Validate(); err != nil {
		return model.Trip{}, err
	}
	record := model.Trip{Data: data, ID: uuid.New(), Status: trip.StatusActive, StartedAt: time.Now().UTC()}
	err := u.transactions.Do(ctx, func(ctx context.Context) error {
		if err := u.repository.Create(ctx, record); err != nil {
			return err
		}
		return u.repository.AddHistory(ctx, record.ID, nil, string(trip.StatusActive))
	})
	if err != nil {
		return model.Trip{}, fmt.Errorf("create trip: %w", err)
	}
	u.metrics.Created(ctx)
	u.logger.InfoContext(ctx, "trip created", "trip_id", record.ID)
	return record, nil
}
