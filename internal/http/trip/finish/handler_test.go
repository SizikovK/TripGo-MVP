package finish_test

import (
	"encoding/json"
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
	api "github.com/SizikovK/TripGo-MVP/internal/generated"
	"github.com/SizikovK/TripGo-MVP/internal/http/trip/finish"
	"github.com/SizikovK/TripGo-MVP/internal/model"
)

func TestHandle(t *testing.T) {
	for _, tc := range []struct {
		name    string
		failure error
		status  int
	}{
		{name: "success", status: 200},
		{name: "not found", failure: domain.ErrTripNotFound, status: 404},
		{name: "completed", failure: domain.ErrTripCompleted, status: 409},
		{name: "busy", failure: domain.ErrDriverBusy, status: 409},
		{name: "internal error", failure: errors.New("database secret"), status: 500},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			usecase := NewMockusecase(ctrl)
			record := model.Trip{ID: uuid.New(), Status: domain.StatusActive}
			usecase.EXPECT().Execute(gomock.Any(), record.ID).Return(record, tc.failure)
			logger := slog.New(slog.NewTextHandler(io.Discard, nil))
			handler := finish.New(usecase, logger, time.Second)
			request := httptest.NewRequest("POST", "/api/v1/trips/"+record.ID.String(), nil)
			response := httptest.NewRecorder()
			handler.Handle(response, request, record.ID)
			require.Equal(t, tc.status, response.Code)
			if tc.failure != nil {
				require.Equal(t, "application/problem+json", response.Header().Get("Content-Type"))
				var problem api.Problem
				require.NoError(t, json.Unmarshal(response.Body.Bytes(), &problem))
				require.EqualValues(t, tc.status, problem.Status)
				require.NotContains(t, response.Body.String(), "database secret")
			} else {
				require.Equal(t, "application/json", response.Header().Get("Content-Type"))
				var actual api.Trip
				require.NoError(t, json.Unmarshal(response.Body.Bytes(), &actual))
				require.Equal(t, record.ID, actual.Id)
			}
		})
	}
}
