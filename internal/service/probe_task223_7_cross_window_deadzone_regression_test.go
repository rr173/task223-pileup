package service

import (
	"testing"

	"task223-pileup/internal/store"
)

func TestTask223Bug07DeadZonesKeepWindowOrigins(t *testing.T) {
	db, err := store.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	app, err := New(db)
	if err != nil {
		t.Fatal(err)
	}
	run, err := app.Runs.Create("window-origins", "", "NaI", 500000000, 40)
	if err != nil {
		t.Fatal(err)
	}
	saturated := []float64{0.1, 0.1, 0.1, 0.1, 0.1, 0.1, 1.1, 1.1, 1.1, 1.1, 0.1, 0.1}
	for i, start := range []int64{0, 1000} {
		if _, err := app.Windows.Ingest(run, int64(i+1), start, 20, saturated); err != nil {
			t.Fatal(err)
		}
	}
	normal := []float64{0, 0, 0, 0, 0, 0.8, 0, 0, 0, 0, 0, 0}
	if _, err := app.Windows.Ingest(run, 3, 2000, 20, normal); err != nil {
		t.Fatal(err)
	}
	if _, err := app.Runs.FinishReceiving(run.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := app.Deconv.Deconvolve(run.ID); err != nil {
		t.Fatal(err)
	}
	zones, err := app.Deconv.ListDeadZones(run.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(zones) != 2 {
		t.Fatalf("zones = %+v, want one zone per saturated window", zones)
	}
	if zones[0].StartTimeNs != 12 || zones[1].StartTimeNs != 1012 {
		t.Fatalf("zones = %+v, want starts 12ns and 1012ns", zones)
	}
}
