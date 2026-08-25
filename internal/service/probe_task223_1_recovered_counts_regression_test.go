package service

import (
	"testing"
	"time"

	"task223-pileup/internal/model"
	"task223-pileup/internal/store"
)

func TestTask223Bug01RecoveredCountsRegression(t *testing.T) {
	db, err := store.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	app, err := New(db)
	if err != nil {
		t.Fatal(err)
	}
	run, err := app.Runs.Create("recovered-counts", "", "NaI", 5e8, 40)
	if err != nil {
		t.Fatal(err)
	}
	window, err := app.Windows.Ingest(run, 1, 0, 400, []float64{0, 1, 0})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = app.Runs.FinishReceiving(run.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = app.Runs.CompleteProcessing(run.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = app.Runs.Confirm(run.ID); err != nil {
		t.Fatal(err)
	}

	pulseStore := store.NewPulseStore(db)
	now := time.Now().UTC()
	for _, pulse := range []*model.PulseEvent{
		{ID: "isolated", RunID: run.ID, WindowID: window.Window.ID, GroupIndex: 0, Status: model.PulseSeparated, CreatedAt: now, UpdatedAt: now},
		{ID: "recovered", RunID: run.ID, WindowID: window.Window.ID, GroupIndex: 7, Status: model.PulseSeparated, CreatedAt: now, UpdatedAt: now},
	} {
		if err := pulseStore.Create(pulse); err != nil {
			t.Fatal(err)
		}
	}

	snapshot, err := app.Snapshots.Publish(run.ID)
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.TotalCounts != 2 {
		t.Fatalf("total counts = %d, want 2", snapshot.TotalCounts)
	}
	if snapshot.RecoveredCounts != 1 {
		t.Fatalf("recovered counts = %d, want 1 (only the piled pulse)", snapshot.RecoveredCounts)
	}
}
