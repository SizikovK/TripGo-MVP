package trips

import (
	"context"
	"errors"
	"time"

	api "github.com/SizikovK/TripGo-MVP/internal/generated"
	"github.com/google/uuid"
)

var (
	ErrNotFound   = errors.New("поездка не найдена")
	ErrCompleted  = errors.New("поездка уже завершена")
	ErrDriverBusy = errors.New("у водителя уже есть активная поездка")
)

type TxManager interface {
	Do(context.Context, func(context.Context) error) error
}

type Repository interface {
	Create(context.Context, api.Trip) error
	Get(context.Context, uuid.UUID) (api.Trip, error)
	Finish(context.Context, uuid.UUID, time.Time) (api.Trip, error)
	AddHistory(context.Context, uuid.UUID, *string, string) error
}

type Service struct {
	repository Repository
	txManager  TxManager
}

func NewService(repository Repository, txManager TxManager) *Service {
	return &Service{repository: repository, txManager: txManager}
}

func (s *Service) Create(ctx context.Context, data api.TripData) (api.Trip, error) {
	trip := api.Trip{
		Id:         uuid.New(),
		UserId:     data.UserId,
		DriverId:   data.DriverId,
		StartPoint: data.StartPoint,
		EndPoint:   data.EndPoint,
		Price:      data.Price,
		Status:     api.Active,
		StartedAt:  time.Now().UTC(),
	}
	err := s.txManager.Do(ctx, func(ctx context.Context) error {
		if err := s.repository.Create(ctx, trip); err != nil {
			return err
		}
		return s.repository.AddHistory(ctx, trip.Id, nil, string(api.Active))
	})
	return trip, err
}

func (s *Service) Get(ctx context.Context, id uuid.UUID) (api.Trip, error) {
	return s.repository.Get(ctx, id)
}

func (s *Service) Finish(ctx context.Context, id uuid.UUID) (api.Trip, error) {
	var trip api.Trip
	err := s.txManager.Do(ctx, func(ctx context.Context) error {
		var updateErr error
		trip, updateErr = s.repository.Finish(ctx, id, time.Now().UTC())
		if updateErr != nil {
			return updateErr
		}
		from := string(api.Active)
		return s.repository.AddHistory(ctx, id, &from, string(api.Completed))
	})
	return trip, err
}
