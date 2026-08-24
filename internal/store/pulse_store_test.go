package store

import (
	"testing"
	"time"

	"task223-pileup/internal/model"
)

func TestCountRecoveredExcludesIsolatedPulses(t *testing.T) {
	db, err := Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	pulses := []*model.PulseEvent{
		{ID: "p-isolated", RunID: "run-1", WindowID: "w-1", GroupIndex: 0, Status: model.PulseSeparated, CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()},
		{ID: "p-recovered", RunID: "run-1", WindowID: "w-2", GroupIndex: 1, Status: model.PulseSeparated, CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()},
		{ID: "p-other-run", RunID: "run-2", WindowID: "w-3", GroupIndex: 2, Status: model.PulseSeparated, CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()},
	}
	store := NewPulseStore(db)
	for _, pulse := range pulses {
		if err := store.Create(pulse); err != nil {
			t.Fatal(err)
		}
	}
	got, err := store.CountRecovered("run-1")
	if err != nil {
		t.Fatal(err)
	}
	if got != 1 {
		t.Fatalf("recovered count = %d, want 1", got)
	}
}
