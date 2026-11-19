package create_test

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

	"github.com/SizikovK/TripGo-MVP/internal/model"
	"github.com/SizikovK/TripGo-MVP/internal/usecase/position/create"
)

func TestExecute(t *testing.T) {
	for _, name := range []string{"success", "repository error", "commit error", "future time"} {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			repository := NewMockrepository(ctrl)
			transactions := NewMocktransactions(ctrl)
			queue := NewMockqueue(ctrl)
			clock := NewMockclock(ctrl)
			metrics := NewMockmetrics(ctrl)
			ctx := context.Background()
			now := time.Date(2026, 11, 19, 10, 0, 0, 0, time.UTC)
			id := uuid.New()
			data := model.PositionData{RecordedAt: now}
			expected := model.Position{ID: 1, TripID: id, PositionData: data}
			failure := errors.New("database failure")
			clock.EXPECT().Now().Return(now)

			if name == "future time" {
				data.RecordedAt = now.Add(2 * time.Minute)
			} else {
				var repositoryError error
				if name == "repository error" {
					repositoryError = failure
				}

				repository.EXPECT().SavePosition(gomock.Any(), id, data).Return(expected, repositoryError)
				transactions.EXPECT().Do(ctx, gomock.Any()).DoAndReturn(func(ctx context.Context, fn func(context.Context) error) error {
					if err := fn(ctx); err != nil {
						return err
					}
					if name == "commit error" {
						return failure
					}
					return nil
				})
			}

			if name == "success" {
				queue.EXPECT().Cancel(id)
				metrics.EXPECT().Saved(ctx)
			}

			usecase := create.New(repository, transactions, queue, clock, metrics, slog.New(slog.NewTextHandler(io.Discard, nil)))
			actual, err := usecase.Execute(ctx, id, data)
			if name == "success" {
				require.NoError(t, err)
				require.Equal(t, expected, actual)
			} else if name == "future time" {
				require.Error(t, err)
			} else {
				require.ErrorIs(t, err, failure)
			}
		})
	}
}
