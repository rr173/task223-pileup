package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"task223-pileup/internal/model"
	"task223-pileup/internal/service"
	"task223-pileup/internal/store"
)

func TestHandlerCreatesAndListsRunThroughRealMux(t *testing.T) {
	db, err := store.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	app, err := service.New(db)
	if err != nil {
		t.Fatal(err)
	}
	handler := New(app).Handler()

	req := httptest.NewRequest(http.MethodPost, "/api/runs", strings.NewReader(`{"name":"run-a","description":"test","detector_type":"NaI","sample_rate_hz":500000000,"dead_time_ns":40}`))
	req.Header.Set("Content-Type", "application/json")
	create := httptest.NewRecorder()
	handler.ServeHTTP(create, req)
	if create.Code != http.StatusCreated {
		t.Fatalf("create status = %d, want %d: %s", create.Code, http.StatusCreated, create.Body.String())
	}
	var run model.Run
	if err := json.NewDecoder(create.Body).Decode(&run); err != nil {
		t.Fatal(err)
	}
	if run.ID == "" || run.Status != model.RunReceiving {
		t.Fatalf("created run = %+v", run)
	}

	list := httptest.NewRecorder()
	handler.ServeHTTP(list, httptest.NewRequest(http.MethodGet, "/api/runs", nil))
	if list.Code != http.StatusOK {
		t.Fatalf("list status = %d, want %d", list.Code, http.StatusOK)
	}
	var runs []model.Run
	if err := json.NewDecoder(list.Body).Decode(&runs); err != nil {
		t.Fatal(err)
	}
	if len(runs) != 1 || runs[0].ID != run.ID {
		t.Fatalf("listed runs = %+v, want the created run", runs)
	}
}

func TestHandlerMapsMissingRunToNotFound(t *testing.T) {
	db, err := store.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	app, err := service.New(db)
	if err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	New(app).Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/runs/missing", nil))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("missing run status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}
