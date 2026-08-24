package service

import (
	"testing"

	"task223-pileup/internal/store"
)

func TestTask223Bug05ReferenceSkipsPiledWindow(t *testing.T) {
	db, err := store.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	app, err := New(db)
	if err != nil {
		t.Fatal(err)
	}
	run, err := app.Runs.Create("reference-source", "", "NaI", 500000000, 40)
	if err != nil {
		t.Fatal(err)
	}
	piled := make([]float64, 40)
	piled[5], piled[15] = 0.8, 0.7
	isolated := make([]float64, 40)
	isolated[7] = 0.9
	if _, err := app.Windows.Ingest(run, 1, 0, 80, piled); err != nil {
		t.Fatal(err)
	}
	second, err := app.Windows.Ingest(run, 2, 80, 80, isolated)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := app.Runs.FinishReceiving(run.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := app.Deconv.Deconvolve(run.ID); err != nil {
		t.Fatal(err)
	}
	ref, err := app.Deconv.baselineStore.GetReferencePulse(run.ID)
	if err != nil {
		t.Fatal(err)
	}
	if ref.SourceWindow != second.Window.ID {
		t.Fatalf("reference source = %q, want isolated window %q", ref.SourceWindow, second.Window.ID)
	}
	if ref.SourceWindow == "" {
		t.Fatal("reference source should be a concrete isolated window")
	}
}
