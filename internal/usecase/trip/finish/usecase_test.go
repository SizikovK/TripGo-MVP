package finish_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/SizikovK/TripGo-MVP/internal/domain/trip"
	"github.com/SizikovK/TripGo-MVP/internal/model"
	"github.com/SizikovK/TripGo-MVP/internal/usecase/trip/finish"
)

func TestExecute(t *testing.T) {
	failure := errors.New("database secret")
	for _, tc := range []struct {
		name                                 string
		getErr, finishErr, historyErr, txErr error
		completed                            bool
	}{
		{name: "success"},
		{name: "not found", getErr: trip.ErrTripNotFound},
		{name: "get error", getErr: failure},
		{name: "completed", completed: true},
		{name: "concurrent finish", finishErr: trip.ErrTripCompleted},
		{name: "update error", finishErr: failure},
		{name: "history error", historyErr: failure},
		{name: "transaction error", txErr: failure},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			repository := NewMocktripRepository(ctrl)
			transactions := NewMocktxManager(ctrl)
			metrics := NewMockmetrics(ctrl)
			record := model.Trip{ID: uuid.New(), Status: trip.StatusActive, StartedAt: time.Now().UTC()}
			if tc.completed {
				record.Status = trip.StatusCompleted
			}
			if tc.txErr != nil {
				transactions.EXPECT().Do(gomock.Any(), gomock.Any()).Return(tc.txErr)
			} else {
				transactions.EXPECT().Do(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, fn func(context.Context) error) error { return fn(ctx) })
				repository.EXPECT().Get(gomock.Any(), record.ID).Return(record, tc.getErr)
				if tc.getErr == nil && !tc.completed {
					repository.EXPECT().Finish(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, updated model.Trip) error {
						require.Equal(t, trip.StatusCompleted, updated.Status)
						require.NotNil(t, updated.FinishedAt)
						require.Equal(t, record.StartedAt, updated.StartedAt)
						return tc.finishErr
					})
					if tc.finishErr == nil {
						repository.EXPECT().AddHistory(gomock.Any(), record.ID, gomock.Any(), string(trip.StatusCompleted)).Return(tc.historyErr)
					}
					if tc.finishErr == nil && tc.historyErr == nil {
						metrics.EXPECT().Completed(gomock.Any())
					}
				}
			}
			usecase := finish.New(repository, transactions, slog.New(slog.NewTextHandler(io.Discard, nil)), metrics)
			actual, err := usecase.Execute(context.Background(), record.ID)
			switch {
			case tc.txErr != nil:
				require.ErrorIs(t, err, tc.txErr)
			case tc.getErr != nil:
				require.ErrorIs(t, err, tc.getErr)
			case tc.completed:
				require.ErrorIs(t, err, trip.ErrTripCompleted)
			case tc.finishErr != nil:
				require.ErrorIs(t, err, tc.finishErr)
			case tc.historyErr != nil:
				require.ErrorIs(t, err, tc.historyErr)
			default:
				require.NoError(t, err)
				require.Equal(t, record.ID, actual.ID)
				require.Equal(t, trip.StatusCompleted, actual.Status)
			}
		})
	}
}
