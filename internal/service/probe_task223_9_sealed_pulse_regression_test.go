package service

import (
	"errors"
	"testing"
	"time"

	"task223-pileup/internal/model"
	"task223-pileup/internal/store"
)

func TestTask223Bug09SealedRunRejectsPulseReviewMutation(t *testing.T) {
	db, err := store.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	app, err := New(db)
	if err != nil {
		t.Fatal(err)
	}
	run, err := app.Runs.Create("sealed-review", "", "NaI", 500000000, 40)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	if err := app.Deconv.pulseStore.Create(&model.PulseEvent{
		ID: "sealed-pulse", RunID: run.ID, WindowID: "w", Status: model.PulseSeparated,
		CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		t.Fatal(err)
	}
	for _, next := range []string{model.RunProcessing, model.RunPendingReview, model.RunCompleted, model.RunSealed} {
		current, err := app.Runs.Get(run.ID)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := app.Runs.store.UpdateStatus(run.ID, current.Status, next); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := app.Deconv.ConfirmPulse("sealed-pulse"); !errors.Is(err, model.ErrSealed) {
		t.Fatalf("confirming pulse in sealed run error = %v, want ErrSealed", err)
	}
}
