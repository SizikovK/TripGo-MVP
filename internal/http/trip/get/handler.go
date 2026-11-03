package get

import (
	"context"
	"github.com/SizikovK/TripGo-MVP/internal/http/response"
	"github.com/SizikovK/TripGo-MVP/internal/http/trip"
	"github.com/google/uuid"
	"log/slog"
	"net/http"
	"time"
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
	record, err := h.usecase.Execute(ctx, id)
	if err != nil {
		response.Error(w, r, h.logger, err)
		return
	}
	response.JSON(w, r, h.logger, http.StatusOK, trip.Response(record))
}
