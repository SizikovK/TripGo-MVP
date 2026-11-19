package list

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"

	api "github.com/SizikovK/TripGo-MVP/internal/generated"
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
	ctx, cancel := context.WithTimeout(r.Context(), h.timeout)
	defer cancel()

	positions, err := h.usecase.Execute(ctx, id)
	if err != nil {
		response.Error(w, r, h.logger, err)
		return
	}

	output := api.TripPositionsResponse{Positions: make([]api.TripPosition, 0, len(positions))}
	for _, position := range positions {
		output.Positions = append(output.Positions, trip.PositionResponse(position))
	}

	response.JSON(w, r, h.logger, http.StatusOK, output)
}
