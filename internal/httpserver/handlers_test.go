package httpserver

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/SizikovK/TripGo-MVP/internal/config"
)

func TestDecodeTrip(t *testing.T) {
	valid := `{"user_id":"5cb72c04-7650-45c9-a79b-bcdba0631e0c","driver_id":"8860b315-ec86-42eb-a17c-7c163d721ff5","start_point":{"latitude":0,"longitude":0},"end_point":{"latitude":90,"longitude":-180},"price":0}`
	for _, tc := range []struct {
		name, body string
		valid      bool
	}{
		{"zero values are valid", valid, true},
		{"missing price", strings.Replace(valid, `,"price":0`, "", 1), false},
		{"missing latitude", strings.Replace(valid, `"latitude":0,`, "", 1), false},
		{"null price", strings.Replace(valid, `"price":0`, `"price":null`, 1), false},
		{"null point", strings.Replace(valid, `{"latitude":0,"longitude":0}`, "null", 1), false},
		{"unknown nested field", strings.Replace(valid, `"latitude":0`, `"extra":1,"latitude":0`, 1), false},
		{"invalid UUID", strings.Replace(valid, "5cb72c04-7650-45c9-a79b-bcdba0631e0c", "bad", 1), false},
		{"negative price", strings.Replace(valid, `"price":0`, `"price":-1`, 1), false},
		{"out of range", strings.Replace(valid, `"latitude":90`, `"latitude":91`, 1), false},
		{"second object", valid + ` {}`, false},
		{"null body", "null", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodPost, "/api/v1/trips", strings.NewReader(tc.body))
			r.Header.Set("Content-Type", "application/json")
			_, err := decodeTrip(httptest.NewRecorder(), r)
			if (err == nil) != tc.valid {
				t.Fatalf("valid=%v, error=%v", tc.valid, err)
			}
		})
	}
}

func TestInvalidPathParameterUsesProblemJSON(t *testing.T) {
	server := NewServer(&config.Config{}, nil, nil)
	w := httptest.NewRecorder()
	server.Handler.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/trips/not-a-uuid", nil))
	if w.Code != 400 || w.Header().Get("Content-Type") != "application/problem+json" || !strings.Contains(w.Body.String(), `"code":"invalid_request"`) {
		t.Fatalf("unexpected response: %d %s", w.Code, w.Body.String())
	}
}
