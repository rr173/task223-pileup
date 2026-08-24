package service

import (
	"testing"

	"task223-pileup/internal/store"
)

func TestTask223Bug03RunLevelBaselineSlopeCreatesDeadZone(t *testing.T) {
	db, err := store.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	app, err := New(db)
	if err != nil {
		t.Fatal(err)
	}
	run, err := app.Runs.Create("drifting-baseline", "", "NaI", 500000000, 40)
	if err != nil {
		t.Fatal(err)
	}
	levels := []float64{0.00, 0.03, 0.06, 0.09}
	for i, level := range levels {
		samples := []float64{level, level, level, level, level, level, level, level, level, level, level, level}
		if i == 0 {
			samples[5] = level + 0.6
		}
		if _, err := app.Windows.Ingest(run, int64(i+1), int64(i*100), 20, samples); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := app.Runs.FinishReceiving(run.ID); err != nil {
		t.Fatal(err)
	}
	res, err := app.Deconv.Deconvolve(run.ID)
	if err != nil {
		t.Fatal(err)
	}
	if res.DeadZones == 0 {
		t.Fatalf("run-level baseline slope was ignored: result=%+v", res)
	}
	deadzones, err := app.Deconv.ListDeadZones(run.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(deadzones) == 0 || deadzones[0].Reason != "baseline_drift" {
		t.Fatalf("deadzones = %+v, want baseline_drift", deadzones)
	}
}
