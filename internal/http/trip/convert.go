package trip

import (
	"bytes"
	"encoding/json"
	"io"
	"mime"
	"net/http"

	domain "github.com/SizikovK/TripGo-MVP/internal/domain/trip"
	api "github.com/SizikovK/TripGo-MVP/internal/generated"
	"github.com/SizikovK/TripGo-MVP/internal/model"
)

const bodyField = "body"

func Decode(w http.ResponseWriter, r *http.Request) (model.CreateTrip, error) {
	var input api.CreateTripJSONRequestBody
	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		return model.CreateTrip{}, &domain.ValidationError{Field: "Content-Type", Detail: "must be application/json"}
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return model.CreateTrip{}, &domain.ValidationError{Field: bodyField, Detail: "cannot read body or size exceeds 1 MiB"}
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		return model.CreateTrip{}, &domain.ValidationError{Field: bodyField, Detail: "invalid JSON or unknown fields"}
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return model.CreateTrip{}, &domain.ValidationError{Field: bodyField, Detail: "expected one JSON object"}
	}
	fields, err := requiredFields(body, "", "user_id", "driver_id", "start_point", "end_point", "price")
	if err != nil {
		return model.CreateTrip{}, err
	}
	for _, name := range []string{"start_point", "end_point"} {
		if _, err := requiredFields(fields[name], name+".", "latitude", "longitude"); err != nil {
			return model.CreateTrip{}, err
		}
	}
	data := model.CreateTrip{
		UserID: input.UserId, DriverID: input.DriverId, Price: input.Price,
		StartPoint: model.Coordinates{Latitude: input.StartPoint.Latitude, Longitude: input.StartPoint.Longitude},
		EndPoint:   model.Coordinates{Latitude: input.EndPoint.Latitude, Longitude: input.EndPoint.Longitude},
	}
	if err := data.Validate(); err != nil {
		return model.CreateTrip{}, err
	}
	return data, nil
}

func requiredFields(body []byte, prefix string, names ...string) (map[string]json.RawMessage, error) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(body, &fields); err != nil {
		return nil, &domain.ValidationError{Field: bodyField, Detail: "expected an object"}
	}
	for _, name := range names {
		value, ok := fields[name]
		if !ok || bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
			return nil, &domain.ValidationError{Field: prefix + name, Detail: "required"}
		}
	}
	return fields, nil
}

func Response(record model.Trip) api.Trip {
	return api.Trip{
		Id: record.ID, UserId: record.UserID, DriverId: record.DriverID, Price: record.Price,
		Status: api.TripStatus(record.Status), StartedAt: record.StartedAt, FinishedAt: record.FinishedAt,
		StartPoint: api.Coordinates{Latitude: record.StartPoint.Latitude, Longitude: record.StartPoint.Longitude},
		EndPoint:   api.Coordinates{Latitude: record.EndPoint.Latitude, Longitude: record.EndPoint.Longitude},
	}
}
