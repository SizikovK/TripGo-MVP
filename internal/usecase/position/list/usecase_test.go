package list_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/SizikovK/TripGo-MVP/internal/model"
	"github.com/SizikovK/TripGo-MVP/internal/usecase/position/list"
)

func TestExecute(t *testing.T) {
	for _, failure := range []error{nil, errors.New("database failure")} {
		ctrl := gomock.NewController(t)
		repository := NewMockrepository(ctrl)
		id := uuid.New()
		expected := []model.Position{{ID: 42, TripID: id}}
		repository.EXPECT().ListPositions(gomock.Any(), id).Return(expected, failure)

		actual, err := list.New(repository).Execute(context.Background(), id)
		require.Equal(t, expected, actual)
		require.ErrorIs(t, err, failure)
	}
}
