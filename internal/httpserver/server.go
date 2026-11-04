package httpserver

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/SizikovK/TripGo-MVP/internal/config"
	api "github.com/SizikovK/TripGo-MVP/internal/generated"
	"github.com/SizikovK/TripGo-MVP/internal/trips"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"
)

func NewServer(config *config.Config, dbPool *pgxpool.Pool, service *trips.Service) *http.Server {
	router := chi.NewRouter()
	router.Use(logRequests)
	apiHandler := NewHTTPHandler(config, dbPool, service)
	api.HandlerWithOptions(apiHandler, api.ChiServerOptions{
		BaseRouter: router,
		ErrorHandlerFunc: func(w http.ResponseWriter, r *http.Request, err error) {
			writeProblem(w, r, http.StatusBadRequest, "invalid_request", "Invalid request", "Invalid request parameter")
		},
	})

	return &http.Server{
		Addr:              config.HttpAddr,
		Handler:           router,
		ReadTimeout:       config.ServerReadTimeout,
		ReadHeaderTimeout: config.ServerReadHeaderTimeout,
		WriteTimeout:      config.ServerWriteTimeout,
		IdleTimeout:       config.ServerIdleTimeout,
	}
}

func logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		writer := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
		defer func() {
			status := writer.Status()
			if status == 0 {
				status = http.StatusOK
			}
			slog.Info("HTTP-запрос", "method", r.Method, "path", r.URL.Path,
				"status", status, "bytes", writer.BytesWritten(), "duration", time.Since(started))
		}()
		next.ServeHTTP(writer, r)
	})
}
