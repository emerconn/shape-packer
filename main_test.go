package main

import "testing"

func TestParseArgsAllowsOptionsAfterPositionals(t *testing.T) {
	cfg, err := parseArgs([]string{"3", "4", "6", "--attempts", "7", "--tolerance=1e-6", "--finalstep", "0.001", "--cpuprofile"})
	if err != nil {
		t.Fatalf("parseArgs returned error: %v", err)
	}
	if cfg.innerPolygons != 3 || cfg.innerSides != 4 || cfg.containerSides != 6 {
		t.Fatalf("unexpected positional args: %#v", cfg)
	}
	if cfg.attempts != 7 {
		t.Fatalf("attempts = %d, want 7", cfg.attempts)
	}
	if cfg.penaltyTolerance != 1e-6 {
		t.Fatalf("tolerance = %g, want 1e-6", cfg.penaltyTolerance)
	}
	if cfg.finalStepSize != 0.001 {
		t.Fatalf("finalstep = %g, want 0.001", cfg.finalStepSize)
	}
	if !cfg.cpuProfile {
		t.Fatalf("cpuProfile = false, want true")
	}
}
