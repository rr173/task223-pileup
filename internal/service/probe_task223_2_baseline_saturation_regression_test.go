package service

import (
	"testing"

	"task223-pileup/internal/model"
	"task223-pileup/internal/store"
)

func TestTask223Bug02BaselineOffsetDoesNotFakeSaturation(t *testing.T) {
	db, err := store.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	app, err := New(db)
	if err != nil {
		t.Fatal(err)
	}
	run, err := app.Runs.Create("baseline-offset", "", "NaI", 500000000, 40)
	if err != nil {
		t.Fatal(err)
	}
	res, err := app.Windows.Ingest(run, 1, 0, 12, []float64{0.94, 0.95, 0.99, 0.99, 0.99, 0.99, 0.95, 0.94})
	if err != nil {
		t.Fatal(err)
	}
	if res.Window == nil || res.Inserted != 1 {
		t.Fatalf("ingest result = %+v", res)
	}
	if res.Window.Saturated || res.Window.Status != model.WindowRaw {
		t.Fatalf("high DC baseline was classified as saturation: %+v", res.Window)
	}
}
