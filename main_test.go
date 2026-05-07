package main

import (
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

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

func TestParseArgsDefaults(t *testing.T) {
	cfg, err := parseArgs([]string{"1", "3", "4"})
	if err != nil {
		t.Fatalf("parseArgs returned error: %v", err)
	}
	if cfg.attempts != defaultAttempts {
		t.Fatalf("attempts = %d, want %d", cfg.attempts, defaultAttempts)
	}
	if cfg.penaltyTolerance != defaultPenaltyTolerance {
		t.Fatalf("tolerance = %g, want %g", cfg.penaltyTolerance, defaultPenaltyTolerance)
	}
	if cfg.finalStepSize != defaultFinalStepSize {
		t.Fatalf("finalstep = %g, want %g", cfg.finalStepSize, defaultFinalStepSize)
	}
	if cfg.cpuProfile {
		t.Fatalf("cpuProfile = true, want false")
	}
}

func TestParseArgsHelp(t *testing.T) {
	_, err := parseArgs([]string{"-h"})
	if err == nil {
		t.Fatal("expected error for -h")
	}
	if err.Error() != "help requested" {
		t.Fatalf("error = %q, want help requested", err.Error())
	}

	_, err = parseArgs([]string{"--help"})
	if err == nil {
		t.Fatal("expected error for --help")
	}
	if err.Error() != "help requested" {
		t.Fatalf("error = %q, want help requested", err.Error())
	}
}

func TestParseArgsUnknownOption(t *testing.T) {
	_, err := parseArgs([]string{"1", "3", "4", "--bogus"})
	if err == nil || !strings.Contains(err.Error(), "unknown option") {
		t.Fatalf("error = %v, want unknown option", err)
	}
}

func TestParseArgsMissingPositionals(t *testing.T) {
	cases := []struct {
		args []string
		want string
	}{
		{[]string{}, "expected 3 positional arguments"},
		{[]string{"1"}, "expected 3 positional arguments"},
		{[]string{"1", "3"}, "expected 3 positional arguments"},
	}
	for _, tc := range cases {
		_, err := parseArgs(tc.args)
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Fatalf("args=%v error = %v, want %q", tc.args, err, tc.want)
		}
	}
}

func TestParseArgsInvalidInnerPolygonCount(t *testing.T) {
	_, err := parseArgs([]string{"0", "3", "4"})
	if err == nil || !strings.Contains(err.Error(), "inner polygon count must be positive") {
		t.Fatalf("error = %v, want positive message", err)
	}
}

func TestParseArgsNegativeInnerPolygonCount(t *testing.T) {
	_, err := parseArgs([]string{"-1", "3", "4"})
	if err == nil {
		t.Fatal("expected error for negative inner polygon count")
	}
}

func TestParseArgsInvalidInnerSides(t *testing.T) {
	_, err := parseArgs([]string{"1", "2", "4"})
	if err == nil || !strings.Contains(err.Error(), "inner polygon side count must be at least 3") {
		t.Fatalf("error = %v", err)
	}
}

func TestParseArgsInvalidContainerSides(t *testing.T) {
	_, err := parseArgs([]string{"1", "3", "2"})
	if err == nil || !strings.Contains(err.Error(), "container polygon side count must be at least 3") {
		t.Fatalf("error = %v", err)
	}
}

func TestParseArgsInvalidAttemptsZero(t *testing.T) {
	_, err := parseArgs([]string{"1", "3", "4", "--attempts", "0"})
	if err == nil || !strings.Contains(err.Error(), "--attempts must be positive") {
		t.Fatalf("error = %v", err)
	}
}

func TestParseArgsNegativeTolerance(t *testing.T) {
	_, err := parseArgs([]string{"1", "3", "4", "--tolerance=-1"})
	if err == nil || !strings.Contains(err.Error(), "--tolerance must be non-negative") {
		t.Fatalf("error = %v", err)
	}
}

func TestParseArgsFinalStepOutOfBounds(t *testing.T) {
	cases := []struct {
		val  string
		want string
	}{
		{"0", "--finalstep must be between 0 and 1"},
		{"1", "--finalstep must be between 0 and 1"},
		{"-0.5", "--finalstep must be between 0 and 1"},
		{"1.5", "--finalstep must be between 0 and 1"},
	}
	for _, tc := range cases {
		_, err := parseArgs([]string{"1", "3", "4", "--finalstep", tc.val})
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Fatalf("finalstep=%s error = %v, want %q", tc.val, err, tc.want)
		}
	}
}

func TestParseArgsNonIntegerPositional(t *testing.T) {
	_, err := parseArgs([]string{"abc", "3", "4"})
	if err == nil {
		t.Fatal("expected error for non-integer positional")
	}
}

func TestParseArgsNonIntegerSecondPositional(t *testing.T) {
	_, err := parseArgs([]string{"1", "abc", "4"})
	if err == nil {
		t.Fatal("expected error for non-integer second positional")
	}
}

func TestParseArgsOptionMissingValue(t *testing.T) {
	_, err := parseArgs([]string{"1", "3", "4", "--attempts"})
	if err == nil {
		t.Fatal("expected error for missing --attempts value")
	}
}

func TestParseArgsInvalidAttemptsValue(t *testing.T) {
	_, err := parseArgs([]string{"1", "3", "4", "--attempts", "abc"})
	if err == nil {
		t.Fatal("expected error for non-integer --attempts")
	}
}

func TestParseArgsInvalidToleranceValue(t *testing.T) {
	_, err := parseArgs([]string{"1", "3", "4", "--tolerance", "abc"})
	if err == nil {
		t.Fatal("expected error for non-float --tolerance")
	}
}

func TestParseArgsInvalidFinalstepValue(t *testing.T) {
	_, err := parseArgs([]string{"1", "3", "4", "--finalstep", "abc"})
	if err == nil {
		t.Fatal("expected error for non-float --finalstep")
	}
}

func TestUsage(t *testing.T) {
	s := usage()
	if !strings.Contains(s, "polygon_packer") {
		t.Fatal("usage() should contain program name")
	}
	if !strings.Contains(s, "--attempts") {
		t.Fatal("usage() should contain --attempts")
	}
	if !strings.Contains(s, "--tolerance") {
		t.Fatal("usage() should contain --tolerance")
	}
}

func TestLinspaceSingleValue(t *testing.T) {
	result := linspace(5.0, 10.0, 1)
	if len(result) != 1 || result[0] != 5.0 {
		t.Fatalf("linspace(5, 10, 1) = %v, want [5]", result)
	}
}

func TestLinspaceTwoValues(t *testing.T) {
	result := linspace(0.0, 1.0, 2)
	if len(result) != 2 {
		t.Fatalf("len = %d, want 2", len(result))
	}
	if math.Abs(result[0]) > 1e-12 || math.Abs(result[1]-1) > 1e-12 {
		t.Fatalf("linspace(0, 1, 2) = %v, want [0, 1]", result)
	}
}

func TestLinspaceMultipleValues(t *testing.T) {
	result := linspace(0.0, 10.0, 5)
	if len(result) != 5 {
		t.Fatalf("len = %d, want 5", len(result))
	}
	expected := []float64{0, 2.5, 5, 7.5, 10}
	for i, v := range result {
		if math.Abs(v-expected[i]) > 1e-12 {
			t.Fatalf("linspace[%d] = %g, want %g", i, v, expected[i])
		}
	}
}

func TestLinspaceNegativeRange(t *testing.T) {
	result := linspace(-2.0, 2.0, 5)
	if len(result) != 5 {
		t.Fatalf("len = %d, want 5", len(result))
	}
	expected := []float64{-2, -1, 0, 1, 2}
	for i, v := range result {
		if math.Abs(v-expected[i]) > 1e-12 {
			t.Fatalf("linspace[%d] = %g, want %g", i, v, expected[i])
		}
	}
}

func TestInitialValuesRandom(t *testing.T) {
	cfg, _ := parseArgs([]string{"3", "4", "6", "--attempts", "1"})
	rng := rand.New(rand.NewSource(42))

	// Force random path by calling multiple times; at least some should be random
	side := 4.0
	for trial := 0; trial < 20; trial++ {
		values := initialValues(rng, cfg, side)
		if len(values) != cfg.innerPolygons*3 {
			t.Fatalf("len(values) = %d, want %d", len(values), cfg.innerPolygons*3)
		}
		for _, v := range values {
			if math.IsNaN(v) || math.IsInf(v, 0) {
				t.Fatalf("value is NaN or Inf: %g", v)
			}
		}
	}
}

func TestInitialValuesGrid(t *testing.T) {
	cfg, _ := parseArgs([]string{"4", "4", "6", "--attempts", "1"})
	rng := rand.New(rand.NewSource(1)) // seed that triggers grid path

	values := initialValues(rng, cfg, 4.0)
	if len(values) != cfg.innerPolygons*3 {
		t.Fatalf("len(values) = %d, want %d", len(values), cfg.innerPolygons*3)
	}
}

func TestUniqueFilenameNoConflict(t *testing.T) {
	dir := t.TempDir()
	base := filepath.Join(dir, "test.png")
	result := uniqueFilename(base)
	if result != base {
		t.Fatalf("uniqueFilename = %q, want %q", result, base)
	}
}

func TestUniqueFilenameWithConflict(t *testing.T) {
	dir := t.TempDir()
	base := filepath.Join(dir, "test.png")
	if err := os.WriteFile(base, []byte{}, 0644); err != nil {
		t.Fatal(err)
	}
	result := uniqueFilename(base)
	expected := filepath.Join(dir, "test_1.png")
	if result != expected {
		t.Fatalf("uniqueFilename = %q, want %q", result, expected)
	}
}

func TestUniqueFilenameMultipleConflicts(t *testing.T) {
	dir := t.TempDir()
	base := filepath.Join(dir, "test.png")
	os.WriteFile(base, []byte{}, 0644)
	os.WriteFile(filepath.Join(dir, "test_1.png"), []byte{}, 0644)
	os.WriteFile(filepath.Join(dir, "test_2.png"), []byte{}, 0644)

	result := uniqueFilename(base)
	expected := filepath.Join(dir, "test_3.png")
	if result != expected {
		t.Fatalf("uniqueFilename = %q, want %q", result, expected)
	}
}

func TestPrecompute(t *testing.T) {
	cfg := &config{
		innerPolygons:  2,
		innerSides:     4,
		containerSides: 6,
	}
	cfg.precompute()

	if len(cfg.unitPolygonVertices) != 4 {
		t.Fatalf("unitPolygonVertices len = %d, want 4", len(cfg.unitPolygonVertices))
	}
	if len(cfg.unitPolygonVectors) != 4 {
		t.Fatalf("unitPolygonVectors len = %d, want 4", len(cfg.unitPolygonVectors))
	}
	if len(cfg.unitContainerVertices) != 6 {
		t.Fatalf("unitContainerVertices len = %d, want 6", len(cfg.unitContainerVertices))
	}
	if len(cfg.unitContainerVectors) != 6 {
		t.Fatalf("unitContainerVectors len = %d, want 6", len(cfg.unitContainerVectors))
	}

	// Unit polygon vertices should be on the unit circle
	for i, v := range cfg.unitPolygonVertices {
		r := math.Sqrt(v.x*v.x + v.y*v.y)
		if math.Abs(r-1) > 1e-12 {
			t.Fatalf("unitPolygonVertices[%d] radius = %g, want 1", i, r)
		}
	}

	// Container apothem should be cos(pi/n)
	expected := math.Cos(math.Pi / 6)
	if math.Abs(cfg.unitContainerApothem-expected) > 1e-12 {
		t.Fatalf("unitContainerApothem = %g, want %g", cfg.unitContainerApothem, expected)
	}
}

func TestParseArgsThirdPositionalInvalid(t *testing.T) {
	_, err := parseArgs([]string{"1", "3", "abc"})
	if err == nil {
		t.Fatal("expected error for non-integer third positional")
	}
}

func TestParseArgsNegativeAttempts(t *testing.T) {
	_, err := parseArgs([]string{"1", "3", "4", "--attempts", "-5"})
	if err == nil || !strings.Contains(err.Error(), "--attempts must be positive") {
		t.Fatalf("error = %v, want positive message", err)
	}
}

func TestParseArgsToleranceZeroAllowed(t *testing.T) {
	cfg, err := parseArgs([]string{"1", "3", "4", "--tolerance=0"})
	if err != nil {
		t.Fatalf("parseArgs returned error: %v", err)
	}
	if cfg.penaltyTolerance != 0 {
		t.Fatalf("tolerance = %g, want 0", cfg.penaltyTolerance)
	}
}

func TestRunAttemptsIntegration(t *testing.T) {
	cfg, _ := parseArgs([]string{"2", "3", "4", "--attempts", "2"})
	results := runAttempts(cfg)
	if len(results) != 2 {
		t.Fatalf("len(results) = %d, want 2", len(results))
	}
	for _, r := range results {
		if math.IsNaN(r.side) || math.IsInf(r.side, 0) {
			t.Fatalf("side = %g, should be finite", r.side)
		}
		if len(r.values) != cfg.innerPolygons*3 {
			t.Fatalf("values len = %d, want %d", len(r.values), cfg.innerPolygons*3)
		}
	}
}

func TestRunAttemptsWorkersCappedByAttempts(t *testing.T) {
	cfg, _ := parseArgs([]string{"1", "3", "4", "--attempts", "1"})
	results := runAttempts(cfg)
	if len(results) != 1 {
		t.Fatalf("len(results) = %d, want 1", len(results))
	}
}

func TestStartCPUProfile(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/cpu.prof"

	stop, err := startCPUProfile(path)
	if err != nil {
		t.Fatalf("startCPUProfile error: %v", err)
	}
	if err := stop(); err != nil {
		t.Fatalf("stop error: %v", err)
	}

	// Verify file was created
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Fatal("profile file not created")
	}
}

func TestStartCPUProfileCreatesDir(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/nested/dir/cpu.prof"

	stop, err := startCPUProfile(path)
	if err != nil {
		t.Fatalf("startCPUProfile error: %v", err)
	}
	if err := stop(); err != nil {
		t.Fatalf("stop error: %v", err)
	}
}

func TestStartCPUProfileStopOnce(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/cpu.prof"

	stop, err := startCPUProfile(path)
	if err != nil {
		t.Fatalf("startCPUProfile error: %v", err)
	}
	if err := stop(); err != nil {
		t.Fatalf("stop error: %v", err)
	}
}
