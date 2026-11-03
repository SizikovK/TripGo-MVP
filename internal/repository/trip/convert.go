package trip

import (
	"github.com/jackc/pgx/v5"

	"github.com/SizikovK/TripGo-MVP/internal/model"
)

func scanTrip(row pgx.Row) (model.Trip, error) {
	var record model.Trip
	err := row.Scan(&record.ID, &record.UserID, &record.DriverID,
		&record.StartPoint.Latitude, &record.StartPoint.Longitude,
		&record.EndPoint.Latitude, &record.EndPoint.Longitude,
		&record.Price, &record.Status, &record.StartedAt, &record.FinishedAt)
	return record, err
}
