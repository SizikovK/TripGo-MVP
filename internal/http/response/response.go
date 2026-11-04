package response

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"github.com/SizikovK/TripGo-MVP/internal/http/domainerror"
)

func JSON(w http.ResponseWriter, r *http.Request, logger *slog.Logger, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(value); err != nil {
		logger.ErrorContext(r.Context(), "encode response", "error", err)
	}
}

func Error(w http.ResponseWriter, r *http.Request, logger *slog.Logger, err error) {
	status, problem := domainerror.Problem(err)
	path := r.URL.Path
	problem.Instance = &path
	if status >= http.StatusInternalServerError {
		span := trace.SpanFromContext(r.Context())
		span.RecordError(err)
		span.SetStatus(codes.Error, "internal error")
		span.SetAttributes(attribute.String("error.type", fmt.Sprintf("%T", err)))
		logger.ErrorContext(r.Context(), "request failed", "error", err)
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(problem); err != nil {
		logger.ErrorContext(r.Context(), "encode problem", "error", err)
	}
}
