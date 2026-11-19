package model

import "github.com/google/uuid"

type PositionRequest struct {
	TripID   uuid.UUID
	DriverID uuid.UUID
}
