package service

import (
	"testing"

	"task223-pileup/internal/store"
)

func TestTask223Bug04SaturatedDeadZoneKeepsPhysicalRange(t *testing.T) {
	db, err := store.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	app, err := New(db)
	if err != nil {
		t.Fatal(err)
	}
	run, err := app.Runs.Create("saturation-range", "", "NaI", 500000000, 40)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := app.Windows.Ingest(run, 1, 0, 20, []float64{0.1, 0.1, 0.1, 0.1, 0.1, 0.1, 1.1, 1.1, 1.1, 1.1, 0.1, 0.1}); err != nil {
		t.Fatal(err)
	}
	if _, err := app.Windows.Ingest(run, 2, 20, 20, []float64{0.1, 0.1, 0.1, 0.1, 0.7, 0.9, 0.7, 0.1, 0.1, 0.1, 0.1, 0.1}); err != nil {
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
	if len(zones) != 1 {
		t.Fatalf("zones = %+v, want one saturated zone", zones)
	}
	if zones[0].StartTimeNs != 12 || zones[0].EndTimeNs != 18 {
		t.Fatalf("saturated zone = %+v, want sample range [6,9] => [12,18]ns", zones[0])
	}
}
