package service

import (
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"task223-pileup/internal/model"
	"task223-pileup/internal/store"
)

func TestTask223Bug08ConcurrentSnapshotPublishHasOneWinner(t *testing.T) {
	db, err := store.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	app, err := New(db)
	if err != nil {
		t.Fatal(err)
	}
	run, err := app.Runs.Create("snapshot-race", "", "NaI", 500000000, 40)
	if err != nil {
		t.Fatal(err)
	}
	res, err := app.Windows.Ingest(run, 1, 0, 100, []float64{0, 0, 0, 0, 0, 0.8, 0, 0, 0, 0})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := app.Runs.FinishReceiving(run.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := app.Runs.CompleteProcessing(run.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := app.Runs.Confirm(run.ID); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	if err := app.Snapshots.pulseStore.Create(&model.PulseEvent{
		ID: "pulse-race", RunID: run.ID, WindowID: res.Window.ID, ArrivalTimeNs: 50,
		Amplitude: 0.8, Status: model.PulseConfirmed, CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		t.Fatal(err)
	}

	const participants = 20
	start := make(chan struct{})
	var wg sync.WaitGroup
	results := make(chan error, participants)
	for i := 0; i < participants; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			_, publishErr := app.Snapshots.Publish(run.ID)
			results <- publishErr
		}()
	}
	close(start)
	wg.Wait()
	close(results)

	winners := 0
	for publishErr := range results {
		if publishErr == nil {
			winners++
			continue
		}
		if errors.Is(publishErr, model.ErrSealed) || errors.Is(publishErr, model.ErrInvalidState) {
			continue
		}
		if strings.Contains(publishErr.Error(), "UNIQUE constraint") {
			t.Fatalf("concurrent publish leaked a storage race: %v", publishErr)
		}
		t.Fatalf("unexpected concurrent publish error: %v", publishErr)
	}
	if winners != 1 {
		t.Fatalf("successful publishers = %d, want exactly one", winners)
	}
}
