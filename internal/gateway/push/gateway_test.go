package push_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/trace/noop"

	"github.com/SizikovK/TripGo-MVP/internal/gateway/push"
	pushapi "github.com/SizikovK/TripGo-MVP/internal/generated/push"
	"github.com/SizikovK/TripGo-MVP/internal/model"
)

func TestRequestPosition(t *testing.T) {
	for _, status := range []int{202, 400, 429, 503} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			var calls atomic.Int32
			received := make(chan pushapi.PushRequest, 1)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				if r.Method != http.MethodPost || r.URL.Path != "/api/v1/push" {
					w.WriteHeader(http.StatusNotFound)
					return
				}

				var input pushapi.PushRequest
				if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
					w.WriteHeader(http.StatusBadRequest)
					return
				}
				received <- input
				w.WriteHeader(status)
			}))
			defer server.Close()
			gateway, err := push.New(server.URL, time.Second, noop.NewTracerProvider())
			require.NoError(t, err)
			defer gateway.Close()

			request := model.PositionRequest{TripID: uuid.New(), DriverID: uuid.New()}
			err = gateway.RequestPosition(context.Background(), request)
			if status == 202 {
				require.NoError(t, err)
			} else {
				var failure *push.StatusError
				require.ErrorAs(t, err, &failure)
				require.Equal(t, status, failure.Code)
			}

			require.EqualValues(t, 1, calls.Load())
			require.Len(t, received, 1)
			input := <-received
			require.Equal(t, pushapi.REQUESTPOSITION, input.Kind)
			require.Equal(t, request.DriverID, input.RecipientId)
			require.Equal(t, request.TripID.String(), (*input.Data)["trip_id"])
			require.NotEmpty(t, input.RequestId)
		})
	}
}

func TestTransportError(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	server.Close()
	gateway, err := push.New(server.URL, time.Second, noop.NewTracerProvider())
	require.NoError(t, err)
	defer gateway.Close()

	err = gateway.RequestPosition(context.Background(), model.PositionRequest{TripID: uuid.New(), DriverID: uuid.New()})
	require.Error(t, err)
	var failure *push.StatusError
	require.NotErrorAs(t, err, &failure)
}

func TestRequestTimeout(t *testing.T) {
	release := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { <-release }))
	defer server.Close()
	defer close(release)

	gateway, err := push.New(server.URL, 20*time.Millisecond, noop.NewTracerProvider())
	require.NoError(t, err)
	defer gateway.Close()

	err = gateway.RequestPosition(context.Background(), model.PositionRequest{TripID: uuid.New(), DriverID: uuid.New()})
	require.Error(t, err)
	var timeout interface{ Timeout() bool }
	require.ErrorAs(t, err, &timeout)
	require.True(t, timeout.Timeout())
}
