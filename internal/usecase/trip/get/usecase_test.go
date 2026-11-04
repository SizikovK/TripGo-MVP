package get_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/SizikovK/TripGo-MVP/internal/domain/trip"
	"github.com/SizikovK/TripGo-MVP/internal/model"
	"github.com/SizikovK/TripGo-MVP/internal/usecase/trip/get"
)

func TestExecute(t *testing.T) {
	for _, failure := range []error{nil, trip.ErrTripNotFound, errors.New("database failure")} {
		ctrl := gomock.NewController(t)
		repository := NewMocktripRepository(ctrl)
		record := model.Trip{ID: uuid.New()}
		repository.EXPECT().Get(gomock.Any(), record.ID).Return(record, failure)
		actual, err := get.New(repository).Execute(context.Background(), record.ID)
		require.ErrorIs(t, err, failure)
		require.Equal(t, record, actual)
	}
}
