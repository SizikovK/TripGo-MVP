package trip_test

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/SizikovK/TripGo-MVP/internal/http/trip"
)

func TestDecode(t *testing.T) {
	valid := `{"user_id":"5cb72c04-7650-45c9-a79b-bcdba0631e0c","driver_id":"8860b315-ec86-42eb-a17c-7c163d721ff5","start_point":{"latitude":0,"longitude":0},"end_point":{"latitude":90,"longitude":180},"price":0}`
	for _, tc := range []struct {
		name, body string
		valid      bool
	}{
		{"zero values", valid, true},
		{"missing price", strings.Replace(valid, `,"price":0`, "", 1), false},
		{"null price", strings.Replace(valid, `"price":0`, `"price":null`, 1), false},
		{"missing coordinate", strings.Replace(valid, `"latitude":0,`, "", 1), false},
		{"unknown field", strings.Replace(valid, `"price":0`, `"extra":1,"price":0`, 1), false},
		{"extra document", valid + ` {}`, false},
		{"null", "null", false},
		{"invalid uuid", strings.Replace(valid, "5cb72c04-7650-45c9-a79b-bcdba0631e0c", "bad", 1), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest("POST", "/api/v1/trips", strings.NewReader(tc.body))
			r.Header.Set("Content-Type", "application/json")
			_, err := trip.Decode(httptest.NewRecorder(), r)
			if tc.valid {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
			}
		})
	}
}
