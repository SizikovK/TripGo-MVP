package trip

import (
	"math"
	"time"

	"github.com/google/uuid"
)

type Status string

const (
	StatusActive    Status = "active"
	StatusCompleted Status = "completed"
)

type Coordinates struct {
	Latitude  float64
	Longitude float64
}

type Data struct {
	UserID     uuid.UUID
	DriverID   uuid.UUID
	StartPoint Coordinates
	EndPoint   Coordinates
	Price      int64
}

type Trip struct {
	Data
	ID         uuid.UUID
	Status     Status
	StartedAt  time.Time
	FinishedAt *time.Time
}

func (d Data) Validate() error {
	if d.UserID == uuid.Nil {
		return &ValidationError{Field: "user_id", Detail: "must be a nonzero UUID"}
	}
	if d.DriverID == uuid.Nil {
		return &ValidationError{Field: "driver_id", Detail: "must be a nonzero UUID"}
	}
	if d.Price < 0 {
		return &ValidationError{Field: "price", Detail: "must be nonnegative"}
	}
	if err := d.StartPoint.Validate("start_point"); err != nil {
		return err
	}
	return d.EndPoint.Validate("end_point")
}

func (c Coordinates) Validate(field string) error {
	if math.IsNaN(c.Latitude) || math.IsInf(c.Latitude, 0) || c.Latitude < -90 || c.Latitude > 90 {
		return &ValidationError{Field: field + ".latitude", Detail: "must be between -90 and 90"}
	}
	if math.IsNaN(c.Longitude) || math.IsInf(c.Longitude, 0) || c.Longitude < -180 || c.Longitude > 180 {
		return &ValidationError{Field: field + ".longitude", Detail: "must be between -180 and 180"}
	}
	return nil
}

func (t *Trip) Finish(at time.Time) error {
	if t.Status != StatusActive {
		return ErrTripCompleted
	}
	t.Status = StatusCompleted
	t.FinishedAt = &at
	return nil
}
