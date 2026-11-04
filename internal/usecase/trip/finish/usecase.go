package finish

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"github.com/SizikovK/TripGo-MVP/internal/domain/trip"
	"github.com/SizikovK/TripGo-MVP/internal/model"
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

func (u *Usecase) Execute(ctx context.Context, id uuid.UUID) (model.Trip, error) {
	var record model.Trip
	err := u.transactions.Do(ctx, func(ctx context.Context) error {
		var err error
		record, err = u.repository.Get(ctx, id)
		if err != nil {
			return err
		}
		if err := record.Finish(time.Now().UTC()); err != nil {
			return err
		}
		if err := u.repository.Finish(ctx, record); err != nil {
			return err
		}
		from := string(trip.StatusActive)
		return u.repository.AddHistory(ctx, id, &from, string(trip.StatusCompleted))
	})
	if err != nil {
		return model.Trip{}, fmt.Errorf("finish trip: %w", err)
	}
	u.metrics.Completed(ctx)
	u.logger.InfoContext(ctx, "trip completed", "trip_id", id)
	return record, nil
}
