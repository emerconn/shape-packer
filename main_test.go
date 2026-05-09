package main

import (
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func testPolygonArgs(innerCount, innerSides, outerSides int, extra ...string) []string {
	args := []string{
		"--inner-count=" + strconv.Itoa(innerCount),
		"--inner-sides=" + strconv.Itoa(innerSides),
		"--outer-sides=" + strconv.Itoa(outerSides),
	}
	args = append(args, extra...)
	return args
}

func testCircleInCircleArgs(innerCount int, extra ...string) []string {
	args := []string{
		"--inner-count=" + strconv.Itoa(innerCount),
		"--inner-sides=c",
		"--outer-sides=c",
	}
	args = append(args, extra...)
	return args
}

func testCircleInPolygonArgs(innerCount, outerSides int, extra ...string) []string {
	args := []string{
		"--inner-count=" + strconv.Itoa(innerCount),
		"--inner-sides=c",
		"--outer-sides=" + strconv.Itoa(outerSides),
	}
	args = append(args, extra...)
	return args
}

func testPolygonInCircleArgs(innerCount, innerSides int, extra ...string) []string {
	args := []string{
		"--inner-count=" + strconv.Itoa(innerCount),
		"--inner-sides=" + strconv.Itoa(innerSides),
		"--outer-sides=c",
	}
	args = append(args, extra...)
	return args
}

func TestParseArgsAllowsOptionsAfterRequired(t *testing.T) {
	cfg, err := parseArgs([]string{
		"--inner-count=3", "--inner-sides=4", "--outer-sides=6",
		"--attempts=7", "--tolerance=1e-6", "--finalstep=0.001", "--cpuprofile",
	})
	if err != nil {
		t.Fatalf("parseArgs returned error: %v", err)
	}
	if cfg.innerCount != 3 || cfg.innerSides != 4 || cfg.containerSides != 6 {
		t.Fatalf("unexpected args: %#v", cfg)
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
	cfg, err := parseArgs(testPolygonArgs(1, 3, 4))
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
	_, err := parseArgs(append(testPolygonArgs(1, 3, 4), "--bogus"))
	if err == nil || !strings.Contains(err.Error(), "unknown option") {
		t.Fatalf("error = %v, want unknown option", err)
	}
}

func TestParseArgsUnexpectedPositional(t *testing.T) {
	_, err := parseArgs([]string{"1", "3", "4"})
	if err == nil || !strings.Contains(err.Error(), "unexpected argument") {
		t.Fatalf("error = %v, want unexpected argument", err)
	}
}

func TestParseArgsInvalidInnerPolygonCount(t *testing.T) {
	_, err := parseArgs(testPolygonArgs(0, 3, 4))
	if err == nil || !strings.Contains(err.Error(), "--inner-count") {
		t.Fatalf("error = %v", err)
	}
}

func TestParseArgsInvalidInnerSides(t *testing.T) {
	_, err := parseArgs(testPolygonArgs(1, 2, 4))
	if err == nil || !strings.Contains(err.Error(), "--inner-sides must be at least 3") {
		t.Fatalf("error = %v", err)
	}
}

func TestParseArgsInvalidContainerSides(t *testing.T) {
	_, err := parseArgs(testPolygonArgs(1, 3, 2))
	if err == nil || !strings.Contains(err.Error(), "--outer-sides must be at least 3") {
		t.Fatalf("error = %v", err)
	}
}

func TestParseArgsInvalidAttemptsZero(t *testing.T) {
	_, err := parseArgs(append(testPolygonArgs(1, 3, 4), "--attempts=0"))
	if err == nil || !strings.Contains(err.Error(), "--attempts must be positive") {
		t.Fatalf("error = %v", err)
	}
}

func TestParseArgsNegativeTolerance(t *testing.T) {
	_, err := parseArgs(append(testPolygonArgs(1, 3, 4), "--tolerance=-1"))
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
		_, err := parseArgs(append(testPolygonArgs(1, 3, 4), "--finalstep="+tc.val))
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Fatalf("finalstep=%s error = %v, want %q", tc.val, err, tc.want)
		}
	}
}

func TestParseArgsOptionMissingValue(t *testing.T) {
	_, err := parseArgs(append(testPolygonArgs(1, 3, 4), "--attempts"))
	if err == nil {
		t.Fatal("expected error for missing --attempts value")
	}
}

func TestParseArgsInvalidAttemptsValue(t *testing.T) {
	_, err := parseArgs(append(testPolygonArgs(1, 3, 4), "--attempts=abc"))
	if err == nil {
		t.Fatal("expected error for non-integer --attempts")
	}
}

func TestParseArgsInvalidToleranceValue(t *testing.T) {
	_, err := parseArgs(append(testPolygonArgs(1, 3, 4), "--tolerance=abc"))
	if err == nil {
		t.Fatal("expected error for non-float --tolerance")
	}
}

func TestParseArgsInvalidFinalstepValue(t *testing.T) {
	_, err := parseArgs(append(testPolygonArgs(1, 3, 4), "--finalstep=abc"))
	if err == nil {
		t.Fatal("expected error for non-float --finalstep")
	}
}

func TestParseArgsMissingInnerSides(t *testing.T) {
	_, err := parseArgs([]string{"--inner-count=3", "--outer-sides=4"})
	if err == nil || !strings.Contains(err.Error(), "--inner-sides is required") {
		t.Fatalf("error = %v", err)
	}
}

func TestParseArgsMissingOuterSides(t *testing.T) {
	_, err := parseArgs([]string{"--inner-count=3", "--inner-sides=4"})
	if err == nil || !strings.Contains(err.Error(), "--outer-sides is required") {
		t.Fatalf("error = %v", err)
	}
}

func TestParseArgsCircleInCircle(t *testing.T) {
	cfg, err := parseArgs(testCircleInCircleArgs(5))
	if err != nil {
		t.Fatalf("parseArgs returned error: %v", err)
	}
	if cfg.innerType != "circle" || cfg.outerType != "circle" {
		t.Fatalf("types = %q/%q, want circle/circle", cfg.innerType, cfg.outerType)
	}
	if cfg.paramsPerShape != 2 {
		t.Fatalf("paramsPerShape = %d, want 2", cfg.paramsPerShape)
	}
}

func TestParseArgsCircleInPolygon(t *testing.T) {
	cfg, err := parseArgs(testCircleInPolygonArgs(4, 6))
	if err != nil {
		t.Fatalf("parseArgs returned error: %v", err)
	}
	if cfg.innerType != "circle" || cfg.outerType != "polygon" {
		t.Fatalf("types = %q/%q, want circle/polygon", cfg.innerType, cfg.outerType)
	}
	if cfg.containerSides != 6 {
		t.Fatalf("containerSides = %d, want 6", cfg.containerSides)
	}
}

func TestParseArgsPolygonInCircle(t *testing.T) {
	cfg, err := parseArgs(testPolygonInCircleArgs(3, 4))
	if err != nil {
		t.Fatalf("parseArgs returned error: %v", err)
	}
	if cfg.innerType != "polygon" || cfg.outerType != "circle" {
		t.Fatalf("types = %q/%q, want polygon/circle", cfg.innerType, cfg.outerType)
	}
	if cfg.innerSides != 4 {
		t.Fatalf("innerSides = %d, want 4", cfg.innerSides)
	}
	if cfg.paramsPerShape != 3 {
		t.Fatalf("paramsPerShape = %d, want 3", cfg.paramsPerShape)
	}
}

func TestUsage(t *testing.T) {
	s := usage()
	if !strings.Contains(s, "polygon-packer") {
		t.Fatal("usage() should contain program name")
	}
	if !strings.Contains(s, "--inner-sides") {
		t.Fatal("usage() should contain --inner-sides")
	}
	if !strings.Contains(s, "--outer-sides") {
		t.Fatal("usage() should contain --outer-sides")
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
	cfg, _ := parseArgs(testPolygonArgs(3, 4, 6, "--attempts=1"))
	rng := rand.New(rand.NewSource(42))

	side := 4.0
	for trial := 0; trial < 20; trial++ {
		values := initialValues(rng, cfg, side)
		if len(values) != cfg.innerCount*cfg.paramsPerShape {
			t.Fatalf("len(values) = %d, want %d", len(values), cfg.innerCount*cfg.paramsPerShape)
		}
		for _, v := range values {
			if math.IsNaN(v) || math.IsInf(v, 0) {
				t.Fatalf("value is NaN or Inf: %g", v)
			}
		}
	}
}

func TestInitialValuesCircle(t *testing.T) {
	cfg, _ := parseArgs(testCircleInCircleArgs(5, "--attempts=1"))
	rng := rand.New(rand.NewSource(42))

	values := initialValues(rng, cfg, 4.0)
	if len(values) != cfg.innerCount*cfg.paramsPerShape {
		t.Fatalf("len(values) = %d, want %d", len(values), cfg.innerCount*cfg.paramsPerShape)
	}
	if cfg.paramsPerShape != 2 {
		t.Fatalf("paramsPerShape = %d, want 2 for circles", cfg.paramsPerShape)
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
	if err := os.WriteFile(base, []byte{}, 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "test_1.png"), []byte{}, 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "test_2.png"), []byte{}, 0644); err != nil {
		t.Fatal(err)
	}

	result := uniqueFilename(base)
	expected := filepath.Join(dir, "test_3.png")
	if result != expected {
		t.Fatalf("uniqueFilename = %q, want %q", result, expected)
	}
}

func TestPrecomputePolygon(t *testing.T) {
	cfg := &config{
		innerType:      "polygon",
		outerType:      "polygon",
		innerCount:     2,
		innerSides:     4,
		containerSides: 6,
		paramsPerShape: 3,
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

	for i, v := range cfg.unitPolygonVertices {
		r := math.Sqrt(v.x*v.x + v.y*v.y)
		if math.Abs(r-1) > 1e-12 {
			t.Fatalf("unitPolygonVertices[%d] radius = %g, want 1", i, r)
		}
	}

	expected := math.Cos(math.Pi / 6)
	if math.Abs(cfg.unitContainerApothem-expected) > 1e-12 {
		t.Fatalf("unitContainerApothem = %g, want %g", cfg.unitContainerApothem, expected)
	}
}

func TestPrecomputeCircleInCircle(t *testing.T) {
	cfg := &config{
		innerType:      "circle",
		outerType:      "circle",
		innerCount:     3,
		paramsPerShape: 2,
	}
	cfg.precompute()

	if len(cfg.unitPolygonVertices) != 0 {
		t.Fatalf("unitPolygonVertices should be empty for circle inner, got %d", len(cfg.unitPolygonVertices))
	}
	if len(cfg.unitContainerVertices) != 0 {
		t.Fatalf("unitContainerVertices should be empty for circle outer, got %d", len(cfg.unitContainerVertices))
	}
}

func TestRunAttemptsIntegration(t *testing.T) {
	cfg, _ := parseArgs(testPolygonArgs(2, 3, 4, "--attempts=2"))
	results := runAttempts(cfg)
	if len(results) != 2 {
		t.Fatalf("len(results) = %d, want 2", len(results))
	}
	for _, r := range results {
		if math.IsNaN(r.side) || math.IsInf(r.side, 0) {
			t.Fatalf("side = %g, should be finite", r.side)
		}
		if len(r.values) != cfg.innerCount*cfg.paramsPerShape {
			t.Fatalf("values len = %d, want %d", len(r.values), cfg.innerCount*cfg.paramsPerShape)
		}
	}
}

func TestRunAttemptsCircleInCircle(t *testing.T) {
	cfg, _ := parseArgs(testCircleInCircleArgs(3, "--attempts=2"))
	results := runAttempts(cfg)
	if len(results) != 2 {
		t.Fatalf("len(results) = %d, want 2", len(results))
	}
	for _, r := range results {
		if math.IsNaN(r.side) || math.IsInf(r.side, 0) {
			t.Fatalf("side = %g, should be finite", r.side)
		}
		if len(r.values) != cfg.innerCount*cfg.paramsPerShape {
			t.Fatalf("values len = %d, want %d", len(r.values), cfg.innerCount*cfg.paramsPerShape)
		}
	}
}

func TestRunAttemptsWorkersCappedByAttempts(t *testing.T) {
	cfg, _ := parseArgs(testPolygonArgs(1, 3, 4, "--attempts=1"))
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
