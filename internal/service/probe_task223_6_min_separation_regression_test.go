package service

import (
	"testing"

	"task223-pileup/internal/deconv"
)

func TestTask223Bug06ConstraintsRemoveTooCloseRecovery(t *testing.T) {
	appConstraints := deconv.DefaultConstraints()
	got := appConstraints.Filter([]deconv.RecoveredPulse{
		{Position: 20, Amplitude: 0.7},
		{Position: 22, Amplitude: 0.4},
		{Position: 40, Amplitude: 0.5},
	})
	if len(got) != 2 {
		t.Fatalf("filtered pulses = %+v, want two separated pulses", got)
	}
	if got[0].Position != 20 || got[1].Position != 40 {
		t.Fatalf("filtered pulses = %+v, want positions 20 and 40", got)
	}
}
