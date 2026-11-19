package trip

import (
	"time"

	"github.com/google/uuid"
)

type PositionData struct {
	Coordinates Coordinates
	RecordedAt  time.Time
}

type Position struct {
	PositionData
	ID     int64
	TripID uuid.UUID
}

func (p PositionData) Validate(now time.Time) error {
	if err := p.Coordinates.Validate("coordinates"); err != nil {
		return err
	}

	if p.RecordedAt.IsZero() || p.RecordedAt.After(now.Add(time.Minute)) {
		return &ValidationError{Field: "recorded_at", Detail: "required and must not exceed current time by more than one minute"}
	}

	return nil
}
