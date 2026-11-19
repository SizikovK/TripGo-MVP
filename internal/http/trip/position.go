package trip

import (
	"net/http"

	api "github.com/SizikovK/TripGo-MVP/internal/generated"
	"github.com/SizikovK/TripGo-MVP/internal/model"
)

func DecodePosition(w http.ResponseWriter, r *http.Request) (model.PositionData, error) {
	var input api.CreateTripPositionJSONRequestBody
	body, err := decodeJSON(w, r, &input)
	if err != nil {
		return model.PositionData{}, err
	}

	fields, err := requiredFields(body, "", "coordinates", "recorded_at")
	if err != nil {
		return model.PositionData{}, err
	}

	if _, err := requiredFields(fields["coordinates"], "coordinates.", "latitude", "longitude"); err != nil {
		return model.PositionData{}, err
	}

	return model.PositionData{
		Coordinates: model.Coordinates{Latitude: input.Coordinates.Latitude, Longitude: input.Coordinates.Longitude},
		RecordedAt:  input.RecordedAt,
	}, nil
}

func PositionResponse(position model.Position) api.TripPosition {
	return api.TripPosition{
		Id: position.ID, TripId: position.TripID, RecordedAt: position.RecordedAt,
		Coordinates: api.Coordinates{Latitude: position.Coordinates.Latitude, Longitude: position.Coordinates.Longitude},
	}
}
