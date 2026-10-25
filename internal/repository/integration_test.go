package repository

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/SizikovK/TripGo-MVP/internal/config"
	api "github.com/SizikovK/TripGo-MVP/internal/generated"
	"github.com/SizikovK/TripGo-MVP/internal/httpserver"
	"github.com/SizikovK/TripGo-MVP/internal/trips"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/joho/godotenv"
	"github.com/pressly/goose/v3"
)

// Uses an isolated schema; no existing application tables are changed.
func TestPostgresTransactionsAndConcurrency(t *testing.T) {
	if os.Getenv("TRIPGO_INTEGRATION") != "1" {
		t.Skip("set TRIPGO_INTEGRATION=1 to test PostgreSQL")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		url = os.Getenv("DATABASE_URL")
	}
	if url == "" {
		env, err := godotenv.Read("../../.env")
		if err != nil {
			t.Fatal("set TEST_DATABASE_URL or provide local .env")
		}
		url = env["DATABASE_URL"]
	}
	admin, err := pgx.Connect(ctx, url)
	if err != nil {
		t.Fatal("test database connection failed")
	}
	defer admin.Close(ctx)
	schema := "tripgo_test_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	quotedSchema := pgx.Identifier{schema}.Sanitize()
	if _, err := admin.Exec(ctx, "CREATE SCHEMA "+quotedSchema); err != nil {
		t.Fatal(err)
	}
	defer func() {
		cleanup, stop := context.WithTimeout(context.Background(), 5*time.Second)
		defer stop()
		if _, err := admin.Exec(cleanup, "DROP SCHEMA "+quotedSchema+" CASCADE"); err != nil {
			t.Error(err)
		}
	}()
	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		t.Fatal(err)
	}
	cfg.ConnConfig.RuntimeParams["search_path"] = schema
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	migrationDB := stdlib.OpenDB(*cfg.ConnConfig)
	defer migrationDB.Close()
	provider, err := goose.NewProvider(goose.DialectPostgres, migrationDB, os.DirFS("../../migrations"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := provider.Up(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := provider.DownTo(ctx, 0); err != nil {
		t.Fatal(err)
	}
	var tables int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM information_schema.tables WHERE table_schema=$1 AND table_name IN ('trips','trip_status_history')", schema).Scan(&tables); err != nil || tables != 0 {
		t.Fatalf("down failed: tables=%d, error=%v", tables, err)
	}
	if _, err := provider.Up(ctx); err != nil {
		t.Fatal(err)
	}
	repo := NewTripRepository(pool)
	tx := NewTxManager(pool, time.Second)
	service := trips.NewService(repo, tx)
	data := api.TripData{UserId: uuid.New(), DriverId: uuid.New(), Price: 100}

	// Nested Do must reuse the same transaction; an outer error rolls it all back.
	id := uuid.New()
	failure := errors.New("intentional failure")
	err = tx.Do(ctx, func(ctx context.Context) error {
		return tx.Do(ctx, func(ctx context.Context) error {
			if err := repo.Create(ctx, api.Trip{Id: id, UserId: data.UserId, DriverId: data.DriverId, Status: api.Active, StartedAt: time.Now()}); err != nil {
				return err
			}
			return failure
		})
	})
	if !errors.Is(err, failure) {
		t.Fatalf("expected rollback error: %v", err)
	}
	if _, err := repo.Get(ctx, id); !errors.Is(err, trips.ErrNotFound) {
		t.Fatalf("rollback failed: %v", err)
	}

	// A failed history insert rolls back a successful trip insert.
	err = tx.Do(ctx, func(ctx context.Context) error {
		if err := repo.Create(ctx, api.Trip{Id: id, UserId: data.UserId, DriverId: data.DriverId, Status: api.Active, StartedAt: time.Now()}); err != nil {
			return err
		}
		return repo.AddHistory(ctx, id, nil, "invalid-status")
	})
	if err == nil {
		t.Fatal("expected history constraint failure")
	}
	if _, err := repo.Get(ctx, id); !errors.Is(err, trips.ErrNotFound) {
		t.Fatalf("partial write: %v", err)
	}

	func() {
		defer func() {
			if recover() != "test panic" {
				t.Error("panic was not propagated")
			}
		}()
		_ = tx.Do(ctx, func(ctx context.Context) error {
			if err := repo.Create(ctx, api.Trip{Id: id, UserId: data.UserId, DriverId: data.DriverId, Status: api.Active, StartedAt: time.Now()}); err != nil {
				t.Fatal(err)
			}
			panic("test panic")
		})
	}()
	if _, err := repo.Get(ctx, id); !errors.Is(err, trips.ErrNotFound) {
		t.Fatalf("panic rollback failed: %v", err)
	}

	var wg sync.WaitGroup
	results := make(chan error, 20)
	created := make(chan api.Trip, 20)
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			trip, err := service.Create(ctx, data)
			if err == nil {
				created <- trip
			}
			results <- err
		}()
	}
	wg.Wait()
	close(results)
	success := 0
	for err := range results {
		if err == nil {
			success++
		} else if !errors.Is(err, trips.ErrDriverBusy) {
			t.Fatal(err)
		}
	}
	if success != 1 {
		t.Fatalf("created %d trips, expected one", success)
	}
	trip := <-created
	results = make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := service.Finish(ctx, trip.Id)
			results <- err
		}()
	}
	wg.Wait()
	close(results)
	success = 0
	for err := range results {
		if err == nil {
			success++
		} else if !errors.Is(err, trips.ErrCompleted) {
			t.Fatal(err)
		}
	}
	if success != 1 {
		t.Fatalf("finished %d times, expected one", success)
	}
	var historyCount int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM trip_status_history WHERE trip_id=$1", trip.Id).Scan(&historyCount); err != nil {
		t.Fatal(err)
	}
	if historyCount != 2 {
		t.Fatalf("history has %d rows, expected two", historyCount)
	}
	finished, err := service.Get(ctx, trip.Id)
	if err != nil || finished.FinishedAt == nil || finished.Status != api.Completed {
		t.Fatalf("invalid completed trip: %+v, %v", finished, err)
	}
	t.Run("HTTP contract", func(t *testing.T) {
		testHTTPContract(t, ctx, cfg, pool, service)
	})
}

func testHTTPContract(t *testing.T, ctx context.Context, cfg *pgxpool.Config, pool *pgxpool.Pool, service *trips.Service) {
	server := httpserver.NewServer(&config.Config{DatabaseQueryTimeout: time.Second}, pool, service)
	request := func(method, path, body string, status int, code string) *httptest.ResponseRecorder {
		t.Helper()
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		server.Handler.ServeHTTP(w, r)
		contentType := "application/json"
		if code != "" {
			contentType = "application/problem+json"
		}
		if w.Code != status || w.Header().Get("Content-Type") != contentType {
			t.Fatalf("%s %s: %d %s", method, path, w.Code, w.Body.String())
		}
		if code != "" {
			var p api.Problem
			if err := json.Unmarshal(w.Body.Bytes(), &p); err != nil || p.Code != code || p.Status != int32(status) {
				t.Fatalf("invalid problem: %s", w.Body.String())
			}
		}
		return w
	}
	body := fmt.Sprintf(`{"user_id":%q,"driver_id":%q,"start_point":{"latitude":0,"longitude":0},"end_point":{"latitude":1,"longitude":1},"price":0}`, uuid.NewString(), uuid.NewString())
	w := request("POST", "/api/v1/trips", body, 201, "")
	var created api.Trip
	if err := json.Unmarshal(w.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	path := "/api/v1/trips/" + created.Id.String()
	if w.Header().Get("Location") != path || created.Status != api.Active || created.StartedAt.IsZero() {
		t.Fatalf("invalid create response: %s", w.Body.String())
	}
	request("GET", path, "", 200, "")
	request("POST", "/api/v1/trips", body, 409, "driver_busy")
	request("POST", path+"/finish", "", 200, "")
	request("POST", path+"/finish", "", 409, "trip_completed")
	request("GET", "/api/v1/trips/"+uuid.NewString(), "", 404, "trip_not_found")
	request("POST", "/api/v1/trips/"+uuid.NewString()+"/finish", "", 404, "trip_not_found")
	request("GET", "/api/v1/trips/bad", "", 400, "invalid_request")
	request("POST", "/api/v1/trips", `{}`, 400, "invalid_request")
	request("GET", "/health", "", 200, "")
	request("GET", "/ready", "", 200, "")
	// Simulate database unavailability without stopping the user's PostgreSQL.
	pool.Close()
	request("GET", "/health", "", 200, "")
	request("GET", "/ready", "", 503, "")
	response := request("GET", path, "", 500, "internal_error")
	if bytes.Contains(response.Body.Bytes(), []byte("closed pool")) {
		t.Fatal("internal error leaked")
	}
	reconnected, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer reconnected.Close()
	server = httpserver.NewServer(&config.Config{DatabaseQueryTimeout: time.Second}, reconnected, service)
	request("GET", "/ready", "", 200, "")
}
