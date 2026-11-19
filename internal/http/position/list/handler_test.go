package list_test

import (
	"errors"
	"io"
	"log/slog"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	domain "github.com/SizikovK/TripGo-MVP/internal/domain/trip"
	"github.com/SizikovK/TripGo-MVP/internal/http/position/list"
	"github.com/SizikovK/TripGo-MVP/internal/model"
)

func TestHandle(t *testing.T) {
	for _, tc := range []struct {
		failure   error
		positions []model.Position
		status    int
	}{
		{status: 200},
		{positions: []model.Position{{ID: 42}}, status: 200},
		{failure: domain.ErrTripNotFound, status: 404},
		{failure: errors.New("database secret"), status: 500},
	} {
		usecase := NewMockusecase(gomock.NewController(t))
		id := uuid.New()
		usecase.EXPECT().Execute(gomock.Any(), id).Return(tc.positions, tc.failure)
		handler := list.New(usecase, slog.New(slog.NewTextHandler(io.Discard, nil)), time.Second)
		response := httptest.NewRecorder()
		handler.Handle(response, httptest.NewRequest("GET", "/positions", nil), id)

		require.Equal(t, tc.status, response.Code)
		require.NotContains(t, response.Body.String(), "database secret")
		if tc.failure == nil && len(tc.positions) == 0 {
			require.JSONEq(t, `{"positions":[]}`, response.Body.String())
		}
	}
}
