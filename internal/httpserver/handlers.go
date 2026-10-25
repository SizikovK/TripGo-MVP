package httpserver

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"strings"
	"time"

	"github.com/SizikovK/TripGo-MVP/internal/config"
	api "github.com/SizikovK/TripGo-MVP/internal/generated"
	"github.com/SizikovK/TripGo-MVP/internal/trips"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Handler struct {
	dbPool       *pgxpool.Pool
	queryTimeout time.Duration
	trips        *trips.Service
}

func NewHTTPHandler(config *config.Config, dbPool *pgxpool.Pool, service *trips.Service) *Handler {
	return &Handler{
		dbPool:       dbPool,
		queryTimeout: config.DatabaseQueryTimeout,
		trips:        service,
	}
}

func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, api.HealthResponse{Status: api.Ok})
}

func (h *Handler) Ready(w http.ResponseWriter, r *http.Request) {
	response := api.HealthResponse{Status: api.Ok}
	status := http.StatusOK

	ctx, cancel := context.WithTimeout(r.Context(), h.queryTimeout)
	defer cancel()

	if err := h.dbPool.Ping(ctx); err != nil {
		slog.Warn("Проверка готовности: БД недоступна", "error", err)
		response = api.HealthResponse{Status: api.Unavailable}
		status = http.StatusServiceUnavailable
	}

	writeJSON(w, status, response)
}

func (h *Handler) CreateTrip(w http.ResponseWriter, r *http.Request, params api.CreateTripParams) {
	data, err := decodeTrip(w, r)
	if err != nil {
		writeProblem(w, r, http.StatusBadRequest, "invalid_request", "Invalid request", err.Error())
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), h.queryTimeout)
	defer cancel()
	trip, err := h.trips.Create(ctx, data)
	if err != nil {
		writeTripError(w, r, err)
		return
	}
	w.Header().Set("Location", "/api/v1/trips/"+trip.Id.String())
	writeJSON(w, http.StatusCreated, trip)
}

func (h *Handler) GetTrip(w http.ResponseWriter, r *http.Request, tripId api.TripId) {
	ctx, cancel := context.WithTimeout(r.Context(), h.queryTimeout)
	defer cancel()
	trip, err := h.trips.Get(ctx, tripId)
	if err != nil {
		writeTripError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, trip)
}

func (h *Handler) FinishTrip(w http.ResponseWriter, r *http.Request, tripId api.TripId) {
	ctx, cancel := context.WithTimeout(r.Context(), h.queryTimeout)
	defer cancel()
	trip, err := h.trips.Finish(ctx, tripId)
	if err != nil {
		writeTripError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, trip)
}

func decodeTrip(w http.ResponseWriter, r *http.Request) (api.TripData, error) {
	var data api.CreateTripJSONRequestBody
	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		return data, errors.New("Content-Type must be application/json")
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return data, errors.New("Unable to read request body (maximum 1 MiB)")
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&data); err != nil {
		return data, errors.New("Invalid JSON or unknown fields")
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return data, errors.New("Expected a single JSON object")
	}
	// Нулевые цена и координаты допустимы, поэтому отдельно проверяем наличие полей.
	fields, err := requiredFields(body, "user_id", "driver_id", "start_point", "end_point", "price")
	if err != nil {
		return data, err
	}
	for _, name := range []string{"start_point", "end_point"} {
		if _, err := requiredFields(fields[name], "latitude", "longitude"); err != nil {
			return data, fmt.Errorf("%s: %w", name, err)
		}
	}
	if data.UserId == uuid.Nil || data.DriverId == uuid.Nil {
		return data, errors.New("user_id and driver_id must be nonzero UUIDs")
	}
	if data.Price < 0 {
		return data, errors.New("price must be nonnegative")
	}
	for _, point := range []api.Coordinates{data.StartPoint, data.EndPoint} {
		if point.Latitude < -90 || point.Latitude > 90 ||
			point.Longitude < -180 || point.Longitude > 180 {
			return data, errors.New("Coordinates are outside allowed ranges")
		}
	}
	return data, nil
}

func requiredFields(body []byte, names ...string) (map[string]json.RawMessage, error) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(body, &fields); err != nil {
		return nil, errors.New("Expected an object")
	}
	for _, name := range names {
		value, ok := fields[name]
		if !ok || bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
			return nil, fmt.Errorf("Missing required field: %s", name)
		}
	}
	return fields, nil
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(value); err != nil {
		slog.Error("Кодирование ответа", "error", err)
	}
}

func writeProblem(w http.ResponseWriter, r *http.Request, status int, code, title, detail string) {
	path := r.URL.Path
	problem := api.Problem{
		Type:     "https://tripgo.example/problems/" + strings.ReplaceAll(code, "_", "-"),
		Title:    title,
		Status:   int32(status),
		Code:     code,
		Detail:   &detail,
		Instance: &path,
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(problem); err != nil {
		slog.Error("Кодирование ошибки", "error", err)
	}
}

func writeTripError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, trips.ErrNotFound):
		writeProblem(w, r, 404, "trip_not_found", "Trip not found", "Trip was not found")
	case errors.Is(err, trips.ErrCompleted):
		writeProblem(w, r, 409, "trip_completed", "Trip completed", "Operation is not allowed for a completed trip")
	case errors.Is(err, trips.ErrDriverBusy):
		writeProblem(w, r, 409, "driver_busy", "Driver busy", "Driver already has an active trip")
	default:
		slog.Error("Обработка поездки", "error", err)
		writeProblem(w, r, 500, "internal_error", "Internal Server Error", "Internal server error")
	}
}
