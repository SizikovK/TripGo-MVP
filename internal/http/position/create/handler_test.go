package create_test

import (
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
	"github.com/SizikovK/TripGo-MVP/internal/http/position/create"
	"github.com/SizikovK/TripGo-MVP/internal/model"
)

func TestHandle(t *testing.T) {
	for _, tc := range []struct {
		failure error
		status  int
	}{
		{status: 201},
		{failure: domain.ErrTripNotFound, status: 404},
		{failure: domain.ErrTripCompleted, status: 409},
		{failure: &domain.ValidationError{Field: "recorded_at", Detail: "future"}, status: 400},
		{failure: errors.New("database secret"), status: 500},
	} {
		ctrl := gomock.NewController(t)
		usecase := NewMockusecase(ctrl)
		id := uuid.New()
		data := model.PositionData{RecordedAt: time.Date(2026, 11, 19, 10, 0, 0, 0, time.UTC)}
		usecase.EXPECT().Execute(gomock.Any(), id, data).Return(model.Position{ID: 42, TripID: id, PositionData: data}, tc.failure)
		handler := create.New(usecase, slog.New(slog.NewTextHandler(io.Discard, nil)), time.Second)

		request := httptest.NewRequest("POST", "/positions", strings.NewReader(`{"coordinates":{"latitude":0,"longitude":0},"recorded_at":"2026-11-19T10:00:00Z"}`))
		request.Header.Set("Content-Type", "application/json")
		response := httptest.NewRecorder()
		handler.Handle(response, request, id)

		require.Equal(t, tc.status, response.Code)
		require.NotContains(t, response.Body.String(), "database secret")
		if tc.failure == nil {
			require.Contains(t, response.Body.String(), `"id":42`)
			require.Contains(t, response.Body.String(), id.String())
		}
	}
}

func TestInvalidJSON(t *testing.T) {
	for _, body := range []string{`{}`, `null`, `{"coordinates":{},"recorded_at":"2026-11-19T10:00:00Z"}`, `{"coordinates":{"latitude":0,"longitude":0},"recorded_at":"bad"}`, `{} {}`} {
		handler := create.New(NewMockusecase(gomock.NewController(t)), slog.New(slog.NewTextHandler(io.Discard, nil)), time.Second)
		request := httptest.NewRequest("POST", "/positions", strings.NewReader(body))
		request.Header.Set("Content-Type", "application/json")
		response := httptest.NewRecorder()
		handler.Handle(response, request, uuid.New())

		require.Equal(t, 400, response.Code)
	}
}
