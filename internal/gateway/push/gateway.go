package push

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/google/uuid"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"

	pushapi "github.com/SizikovK/TripGo-MVP/internal/generated/push"
	"github.com/SizikovK/TripGo-MVP/internal/model"
)

type StatusError struct {
	Code int
}

func (e *StatusError) Error() string {
	return fmt.Sprintf("push service returned HTTP %d", e.Code)
}

type Gateway struct {
	client    *pushapi.Client
	transport *http.Transport
}

func New(endpoint string, timeout time.Duration, traces trace.TracerProvider) (*Gateway, error) {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.ResponseHeaderTimeout = timeout
	transport.TLSHandshakeTimeout = timeout
	client := &http.Client{
		Timeout:       timeout,
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse },
		Transport: otelhttp.NewTransport(transport, otelhttp.WithTracerProvider(traces),
			otelhttp.WithPropagators(propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}))),
	}
	api, err := pushapi.NewClient(endpoint, pushapi.WithHTTPClient(client))
	if err != nil {
		return nil, fmt.Errorf("create push client: %w", err)
	}

	return &Gateway{client: api, transport: transport}, nil
}

func (g *Gateway) RequestPosition(ctx context.Context, request model.PositionRequest) error {
	data := map[string]string{"trip_id": request.TripID.String()}
	response, err := g.client.SendPush(ctx, pushapi.SendPushJSONRequestBody{
		RequestId: uuid.NewString(), RecipientId: request.DriverID, Kind: pushapi.REQUESTPOSITION, Data: &data,
	})
	if err != nil {
		return fmt.Errorf("send position request: %w", err)
	}
	defer response.Body.Close()

	if _, err := io.Copy(io.Discard, io.LimitReader(response.Body, 1<<20)); err != nil {
		return fmt.Errorf("read push response: %w", err)
	}

	if response.StatusCode != http.StatusAccepted {
		return &StatusError{Code: response.StatusCode}
	}

	return nil
}

func (g *Gateway) Close() {
	g.transport.CloseIdleConnections()
}
