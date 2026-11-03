package trip_test

import (
	"errors"
	"math"
	"testing"
	"time"

	"github.com/SizikovK/TripGo-MVP/internal/domain/trip"
	"github.com/google/uuid"
)

func TestValidate(t *testing.T) {
	for _, tc := range []struct {
		name, field string
		change      func(*trip.Data)
	}{
		{name: "valid", change: func(*trip.Data) {}},
		{name: "user", field: "user_id", change: func(d *trip.Data) { d.UserID = uuid.Nil }},
		{name: "driver", field: "driver_id", change: func(d *trip.Data) { d.DriverID = uuid.Nil }},
		{name: "price", field: "price", change: func(d *trip.Data) { d.Price = -1 }},
		{name: "latitude", field: "start_point.latitude", change: func(d *trip.Data) { d.StartPoint.Latitude = 91 }},
		{name: "longitude", field: "end_point.longitude", change: func(d *trip.Data) { d.EndPoint.Longitude = -181 }},
		{name: "nan", field: "end_point.latitude", change: func(d *trip.Data) { d.EndPoint.Latitude = math.NaN() }},
		{name: "infinity", field: "start_point.longitude", change: func(d *trip.Data) { d.StartPoint.Longitude = math.Inf(1) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			data := trip.Data{UserID: uuid.New(), DriverID: uuid.New()}
			tc.change(&data)
			err := data.Validate()
			if tc.field == "" {
				if err != nil {
					t.Fatal(err)
				}
				return
			}
			var validation *trip.ValidationError
			if !errors.As(err, &validation) || validation.Field != tc.field {
				t.Fatalf("unexpected validation error: %v", err)
			}
		})
	}
}

func TestFinish(t *testing.T) {
	at := time.Now().UTC()
	active := trip.Trip{Status: trip.StatusActive}
	if err := active.Finish(at); err != nil {
		t.Fatal(err)
	}
	if active.Status != trip.StatusCompleted || active.FinishedAt == nil || !active.FinishedAt.Equal(at) {
		t.Fatal("trip was not completed")
	}
	if err := active.Finish(at.Add(time.Hour)); !errors.Is(err, trip.ErrTripCompleted) {
		t.Fatalf("unexpected error: %v", err)
	}
	if !active.FinishedAt.Equal(at) {
		t.Fatal("finish time changed")
	}
}
