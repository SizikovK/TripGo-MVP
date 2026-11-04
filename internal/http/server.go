package http

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"

	"github.com/SizikovK/TripGo-MVP/internal/config"
	"github.com/SizikovK/TripGo-MVP/internal/domain/trip"
	api "github.com/SizikovK/TripGo-MVP/internal/generated"
	"github.com/SizikovK/TripGo-MVP/internal/http/middleware"
	"github.com/SizikovK/TripGo-MVP/internal/http/response"
	"github.com/SizikovK/TripGo-MVP/internal/http/trip/create"
	"github.com/SizikovK/TripGo-MVP/internal/http/trip/finish"
	"github.com/SizikovK/TripGo-MVP/internal/http/trip/get"
)

type Dependencies struct {
	Create    *create.Handler
	Get       *get.Handler
	Finish    *finish.Handler
	Readiness pinger
	Logger    *slog.Logger
	Traces    trace.TracerProvider
	Meters    metric.MeterProvider
}

type handlers struct {
	dependencies Dependencies
	timeout      time.Duration
}

func NewServer(cfg *config.Config, dependencies Dependencies) (*http.Server, error) {
	router := chi.NewRouter()
	api.HandlerWithOptions(&handlers{dependencies: dependencies, timeout: cfg.DatabaseQueryTimeout}, api.ChiServerOptions{
		BaseRouter: router,
		ErrorHandlerFunc: func(w http.ResponseWriter, r *http.Request, _ error) {
			response.Error(w, r, dependencies.Logger, &trip.ValidationError{Field: "parameter", Detail: "invalid format"})
		},
	})
	resolveRoute := func(r *http.Request) string {
		ctx := chi.NewRouteContext()
		if !router.Match(ctx, r.Method, r.URL.Path) {
			return "unmatched"
		}
		return ctx.RoutePattern()
	}
	handler, err := middleware.Wrap(router, dependencies.Logger, dependencies.Traces, dependencies.Meters, resolveRoute)
	if err != nil {
		return nil, err
	}
	return &http.Server{
		Addr:              cfg.HttpAddr,
		Handler:           handler,
		ErrorLog:          slog.NewLogLogger(dependencies.Logger.Handler(), slog.LevelError),
		ReadTimeout:       cfg.ServerReadTimeout,
		ReadHeaderTimeout: cfg.ServerReadHeaderTimeout,
		WriteTimeout:      cfg.ServerWriteTimeout,
		IdleTimeout:       cfg.ServerIdleTimeout,
	}, nil
}

func (h *handlers) CreateTrip(w http.ResponseWriter, r *http.Request, _ api.CreateTripParams) {
	h.dependencies.Create.Handle(w, r)
}

func (h *handlers) GetTrip(w http.ResponseWriter, r *http.Request, id api.TripId) {
	h.dependencies.Get.Handle(w, r, id)
}

func (h *handlers) FinishTrip(w http.ResponseWriter, r *http.Request, id api.TripId) {
	h.dependencies.Finish.Handle(w, r, id)
}

func (h *handlers) Health(w http.ResponseWriter, r *http.Request) {
	response.JSON(w, r, h.dependencies.Logger, http.StatusOK, api.HealthResponse{Status: api.Ok})
}

func (h *handlers) Ready(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), h.timeout)
	defer cancel()
	status, result := http.StatusOK, api.Ok
	if err := h.dependencies.Readiness.Ping(ctx); err != nil {
		status, result = http.StatusServiceUnavailable, api.Unavailable
		h.dependencies.Logger.WarnContext(ctx, "database unavailable", "error", err)
	}
	response.JSON(w, r, h.dependencies.Logger, status, api.HealthResponse{Status: result})
}
