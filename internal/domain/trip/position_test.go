package trip_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/SizikovK/TripGo-MVP/internal/domain/trip"
)

func TestPositionValidation(t *testing.T) {
	now := time.Date(2026, 11, 19, 10, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		name    string
		data    trip.PositionData
		invalid bool
	}{
		{name: "current", data: trip.PositionData{RecordedAt: now}},
		{name: "old", data: trip.PositionData{RecordedAt: now.Add(-time.Hour)}},
		{name: "one minute ahead", data: trip.PositionData{RecordedAt: now.Add(time.Minute)}},
		{name: "future", data: trip.PositionData{RecordedAt: now.Add(time.Minute + time.Nanosecond)}, invalid: true},
		{name: "missing time", invalid: true},
		{name: "invalid latitude", data: trip.PositionData{RecordedAt: now, Coordinates: trip.Coordinates{Latitude: 91}}, invalid: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.data.Validate(now)
			if tc.invalid {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
