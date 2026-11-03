package domainerror_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/SizikovK/TripGo-MVP/internal/domain/trip"
	"github.com/SizikovK/TripGo-MVP/internal/http/domainerror"
)

func TestProblem(t *testing.T) {
	for _, tc := range []struct {
		err    error
		status int
		code   string
	}{
		{trip.ErrTripNotFound, 404, "trip_not_found"},
		{trip.ErrTripCompleted, 409, "trip_completed"},
		{trip.ErrDriverBusy, 409, "driver_busy"},
		{&trip.ValidationError{Field: "price", Detail: "negative"}, 400, "invalid_request"},
		{errors.New("database password must not escape"), 500, "internal_error"},
	} {
		t.Run(tc.code, func(t *testing.T) {
			status, body := domainerror.Problem(fmt.Errorf("wrapped: %w", tc.err))
			require.Equal(t, tc.status, status)
			require.Equal(t, tc.code, body.Code)
			require.EqualValues(t, status, body.Status)
			if status == 500 {
				require.NotContains(t, *body.Detail, "password")
			}
		})
	}
}
