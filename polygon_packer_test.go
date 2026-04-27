package main

import "testing"

func TestParseArgsAllowsOptionsAfterPositionals(t *testing.T) {
	cfg, err := parseArgs([]string{"3", "4", "6", "--attempts", "7", "--tolerance=1e-6", "--finalstep", "0.001"})
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
}

func TestPenaltyIsZeroForSingleCenteredPolygonInsideLargeContainer(t *testing.T) {
	cfg, err := parseArgs([]string{"1", "3", "4", "--attempts", "1"})
	if err != nil {
		t.Fatalf("parseArgs returned error: %v", err)
	}
	eval := newEvaluator(cfg)
	values := []float64{0, 0, 0}
	if penalty := eval.value(values, 3); penalty != 0 {
		t.Fatalf("penalty = %g, want 0", penalty)
	}
}

func TestPenaltyIsPositiveForOverlappingPolygons(t *testing.T) {
	cfg, err := parseArgs([]string{"2", "4", "4", "--attempts", "1"})
	if err != nil {
		t.Fatalf("parseArgs returned error: %v", err)
	}
	eval := newEvaluator(cfg)
	values := []float64{0, 0, 0, 0, 0, 0}
	if penalty := eval.value(values, 5); penalty <= 0 {
		t.Fatalf("penalty = %g, want positive", penalty)
	}
}
