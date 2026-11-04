package create_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/SizikovK/TripGo-MVP/internal/domain/trip"
	"github.com/SizikovK/TripGo-MVP/internal/model"
	"github.com/SizikovK/TripGo-MVP/internal/usecase/trip/create"
)

func TestExecute(t *testing.T) {
	failure := errors.New("database secret")
	for _, tc := range []struct {
		name                         string
		createErr, historyErr, txErr error
		invalid                      bool
	}{
		{name: "success"},
		{name: "busy", createErr: trip.ErrDriverBusy},
		{name: "repository error", createErr: failure},
		{name: "history error", historyErr: failure},
		{name: "transaction error", txErr: failure},
		{name: "invalid data", invalid: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			repository := NewMocktripRepository(ctrl)
			transactions := NewMocktxManager(ctrl)
			metrics := NewMockmetrics(ctrl)
			data := model.CreateTrip{UserID: uuid.New(), DriverID: uuid.New(), Price: 100}
			switch {
			case tc.invalid:
				data.Price = -1
			case tc.txErr != nil:
				transactions.EXPECT().Do(gomock.Any(), gomock.Any()).Return(tc.txErr)
			default:
				transactions.EXPECT().Do(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, fn func(context.Context) error) error { return fn(ctx) })
				repository.EXPECT().Create(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, record model.Trip) error {
					require.Equal(t, data, record.Data)
					require.NotEqual(t, uuid.Nil, record.ID)
					require.Equal(t, trip.StatusActive, record.Status)
					require.False(t, record.StartedAt.IsZero())
					return tc.createErr
				})
				if tc.createErr == nil {
					repository.EXPECT().AddHistory(gomock.Any(), gomock.Any(), nil, string(trip.StatusActive)).Return(tc.historyErr)
				}
				if tc.createErr == nil && tc.historyErr == nil {
					metrics.EXPECT().Created(gomock.Any())
				}
			}
			usecase := create.New(repository, transactions, slog.New(slog.NewTextHandler(io.Discard, nil)), metrics)
			record, err := usecase.Execute(context.Background(), data)
			switch {
			case tc.invalid:
				var validation *trip.ValidationError
				require.ErrorAs(t, err, &validation)
			case tc.txErr != nil:
				require.ErrorIs(t, err, tc.txErr)
			case tc.createErr != nil:
				require.ErrorIs(t, err, tc.createErr)
			case tc.historyErr != nil:
				require.ErrorIs(t, err, tc.historyErr)
			default:
				require.NoError(t, err)
				require.Equal(t, data, record.Data)
			}
		})
	}
}
