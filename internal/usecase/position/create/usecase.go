package create

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/google/uuid"

	"github.com/SizikovK/TripGo-MVP/internal/model"
)

type Usecase struct {
	repository   repository
	transactions transactions
	queue        queue
	clock        clock
	metrics      metrics
	logger       *slog.Logger
}

func New(repository repository, transactions transactions, queue queue, clock clock, metrics metrics, logger *slog.Logger) *Usecase {
	return &Usecase{repository: repository, transactions: transactions, queue: queue, clock: clock, metrics: metrics, logger: logger}
}

func (u *Usecase) Execute(ctx context.Context, id uuid.UUID, data model.PositionData) (model.Position, error) {
	if err := data.Validate(u.clock.Now()); err != nil {
		return model.Position{}, err
	}

	var position model.Position
	err := u.transactions.Do(ctx, func(ctx context.Context) error {
		var err error
		position, err = u.repository.SavePosition(ctx, id, data)
		return err
	})
	if err != nil {
		return model.Position{}, fmt.Errorf("save position: %w", err)
	}

	u.queue.Cancel(id)
	u.metrics.Saved(ctx)
	u.logger.DebugContext(ctx, "position saved", "trip_id", id)

	return position, nil
}
