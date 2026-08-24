package service

import (
	"errors"
	"testing"
	"time"

	"task223-pileup/internal/model"
	"task223-pileup/internal/store"
)

func TestTask223Bug10RunCannotCompleteWithUnreviewedPulse(t *testing.T) {
	db, err := store.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	app, err := New(db)
	if err != nil {
		t.Fatal(err)
	}
	run, err := app.Runs.Create("review-gate", "", "NaI", 500000000, 40)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := app.Runs.FinishReceiving(run.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := app.Runs.CompleteProcessing(run.ID); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	if err := app.Deconv.pulseStore.Create(&model.PulseEvent{
		ID: "pending-review", RunID: run.ID, WindowID: "w", Status: model.PulseSeparated,
		CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := app.Runs.Confirm(run.ID); !errors.Is(err, model.ErrConflict) {
		t.Fatalf("confirm with unreviewed pulse error = %v, want ErrConflict", err)
	}
}
