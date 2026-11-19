package create

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/SizikovK/TripGo-MVP/internal/http/response"
	"github.com/SizikovK/TripGo-MVP/internal/http/trip"
)

type Handler struct {
	usecase usecase
	logger  *slog.Logger
	timeout time.Duration
}

func New(usecase usecase, logger *slog.Logger, timeout time.Duration) *Handler {
	return &Handler{usecase: usecase, logger: logger, timeout: timeout}
}

func (h *Handler) Handle(w http.ResponseWriter, r *http.Request) {
	data, err := trip.Decode(w, r)
	if err != nil {
		response.Error(w, r, h.logger, err)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), h.timeout)
	defer cancel()
	record, err := h.usecase.Execute(ctx, data)
	if err != nil {
		response.Error(w, r, h.logger, err)
		return
	}
	w.Header().Set("Location", "/api/v1/trips/"+record.ID.String())
	response.JSON(w, r, h.logger, http.StatusCreated, trip.Response(record))
}
