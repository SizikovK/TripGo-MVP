package domainerror

import (
	"errors"
	"net/http"
	"strings"

	"github.com/SizikovK/TripGo-MVP/internal/domain/trip"
	api "github.com/SizikovK/TripGo-MVP/internal/generated"
)

func Problem(err error) (int, api.Problem) {
	status, code, title, detail := http.StatusInternalServerError, "internal_error", "Internal Server Error", "Internal server error"
	var validation *trip.ValidationError
	switch {
	case errors.As(err, &validation):
		status, code, title, detail = http.StatusBadRequest, "invalid_request", "Invalid request", validation.Error()
	case errors.Is(err, trip.ErrTripNotFound):
		status, code, title, detail = http.StatusNotFound, "trip_not_found", "Trip not found", "Trip was not found"
	case errors.Is(err, trip.ErrTripCompleted):
		status, code, title, detail = http.StatusConflict, "trip_completed", "Trip completed", "Operation is not allowed for a completed trip"
	case errors.Is(err, trip.ErrDriverBusy):
		status, code, title, detail = http.StatusConflict, "driver_busy", "Driver busy", "Driver already has an active trip"
	}
	return status, api.Problem{Type: "https://tripgo.example/problems/" + strings.ReplaceAll(code, "_", "-"), Title: title, Status: int32(status), Code: code, Detail: &detail}
}
