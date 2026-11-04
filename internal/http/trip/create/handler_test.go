package create_test

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	domain "github.com/SizikovK/TripGo-MVP/internal/domain/trip"
	api "github.com/SizikovK/TripGo-MVP/internal/generated"
	"github.com/SizikovK/TripGo-MVP/internal/http/trip/create"
	"github.com/SizikovK/TripGo-MVP/internal/model"
)

func TestHandle(t *testing.T) {
	for _, tc := range []struct {
		name    string
		failure error
		status  int
	}{
		{name: "success", status: 201},
		{name: "not found", failure: domain.ErrTripNotFound, status: 404},
		{name: "completed", failure: domain.ErrTripCompleted, status: 409},
		{name: "busy", failure: domain.ErrDriverBusy, status: 409},
		{name: "internal error", failure: errors.New("database secret"), status: 500},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			usecase := NewMockusecase(ctrl)
			record := model.Trip{ID: uuid.New(), Status: domain.StatusActive}
			usecase.EXPECT().Execute(gomock.Any(), gomock.Any()).Return(record, tc.failure)
			logger := slog.New(slog.NewTextHandler(io.Discard, nil))
			handler := create.New(usecase, logger, time.Second)
			body := `{"user_id":"5cb72c04-7650-45c9-a79b-bcdba0631e0c","driver_id":"8860b315-ec86-42eb-a17c-7c163d721ff5","start_point":{"latitude":0,"longitude":0},"end_point":{"latitude":1,"longitude":1},"price":0}`
			request := httptest.NewRequest("POST", "/api/v1/trips", strings.NewReader(body))
			request.Header.Set("Content-Type", "application/json")
			response := httptest.NewRecorder()
			handler.Handle(response, request)
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
				require.Equal(t, "/api/v1/trips/"+record.ID.String(), response.Header().Get("Location"))
			}
		})
	}
}

func TestInvalidBody(t *testing.T) {
	ctrl := gomock.NewController(t)
	handler := create.New(NewMockusecase(ctrl), slog.New(slog.NewTextHandler(io.Discard, nil)), time.Second)
	request := httptest.NewRequest("POST", "/api/v1/trips", strings.NewReader("{}"))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	handler.Handle(response, request)
	require.Equal(t, 400, response.Code)
	require.Contains(t, response.Body.String(), "invalid_request")
}
