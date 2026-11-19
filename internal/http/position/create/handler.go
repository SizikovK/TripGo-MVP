package create

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"

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

func (h *Handler) Handle(w http.ResponseWriter, r *http.Request, id uuid.UUID) {
	data, err := trip.DecodePosition(w, r)
	if err != nil {
		response.Error(w, r, h.logger, err)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), h.timeout)
	defer cancel()

	position, err := h.usecase.Execute(ctx, id, data)
	if err != nil {
		response.Error(w, r, h.logger, err)
		return
	}

	response.JSON(w, r, h.logger, http.StatusCreated, trip.PositionResponse(position))
}
