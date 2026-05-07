package main

import (
	"math"
	"math/rand"
	"testing"
)

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

func TestOptimizedPenaltyMatchesBruteForceReference(t *testing.T) {
	cases := [][]string{
		{"3", "3", "4", "--attempts", "1"},
		{"4", "5", "6", "--attempts", "1"},
		{"5", "6", "8", "--attempts", "1"},
	}
	rng := rand.New(rand.NewSource(1))

	for _, args := range cases {
		cfg, err := parseArgs(args)
		if err != nil {
			t.Fatalf("parseArgs(%v) returned error: %v", args, err)
		}
		eval := newEvaluator(cfg)

		for trial := 0; trial < 30; trial++ {
			values := make([]float64, cfg.innerPolygons*3)
			for i := range cfg.innerPolygons {
				values[i*3] = rng.Float64()*6 - 3
				values[i*3+1] = rng.Float64()*6 - 3
				values[i*3+2] = rng.Float64() * 2 * math.Pi
			}

			side := 1.5 + rng.Float64()*4
			got := eval.value(values, side)
			want := bruteForcePenalty(cfg, values, side)
			diff := math.Abs(got - want)
			tolerance := 1e-9 * (1 + max(math.Abs(got), math.Abs(want)))
			if diff > tolerance {
				t.Fatalf("penalty mismatch for args %v trial %d: got %g, want %g, diff %g", args, trial, got, want, diff)
			}
		}
	}
}

func TestSpatialPenaltyMatchesBruteForceReference(t *testing.T) {
	cfg, err := parseArgs([]string{"30", "6", "8", "--attempts", "1"})
	if err != nil {
		t.Fatalf("parseArgs returned error: %v", err)
	}
	eval := newEvaluator(cfg)
	rng := rand.New(rand.NewSource(2))

	for trial := 0; trial < 10; trial++ {
		values := make([]float64, cfg.innerPolygons*3)
		for i := range cfg.innerPolygons {
			values[i*3] = rng.Float64()*10 - 5
			values[i*3+1] = rng.Float64()*10 - 5
			values[i*3+2] = rng.Float64() * 2 * math.Pi
		}

		side := 5 + rng.Float64()*4
		got := eval.value(values, side)
		want := bruteForcePenalty(cfg, values, side)
		diff := math.Abs(got - want)
		tolerance := 1e-9 * (1 + max(math.Abs(got), math.Abs(want)))
		if diff > tolerance {
			t.Fatalf("penalty mismatch trial %d: got %g, want %g, diff %g", trial, got, want, diff)
		}
	}
}

func TestIncrementalGradientMatchesFiniteDifference(t *testing.T) {
	cfg, err := parseArgs([]string{"6", "5", "7", "--attempts", "1"})
	if err != nil {
		t.Fatalf("parseArgs returned error: %v", err)
	}
	values := []float64{
		-1.2, -0.9, 0.2,
		0.2, -0.8, 0.9,
		1.1, -0.6, 1.3,
		-0.7, 0.4, 2.0,
		0.6, 0.5, 2.7,
		1.4, 0.7, 3.1,
	}
	side := 3.2

	referenceEval := newEvaluator(cfg)
	f0 := referenceEval.value(values, side)
	want := make([]float64, len(values))
	finiteDifferenceGradient(func(x []float64) float64 {
		return referenceEval.value(x, side)
	}, values, f0, want, len(values))

	incrementalEval := newEvaluator(cfg)
	got := make([]float64, len(values))
	incrementalEval.finiteDifferenceGradient(values, side, f0, got, len(values))

	for i := range got {
		diff := math.Abs(got[i] - want[i])
		tolerance := 1e-7 * (1 + max(math.Abs(got[i]), math.Abs(want[i])))
		if diff > tolerance {
			t.Fatalf("gradient[%d] = %g, want %g, diff %g", i, got[i], want[i], diff)
		}
	}
}

var benchmarkPenalty float64
var benchmarkOptResult optResult

func BenchmarkEvaluatorValue(b *testing.B) {
	cfg, err := parseArgs([]string{"8", "6", "8", "--attempts", "1"})
	if err != nil {
		b.Fatalf("parseArgs returned error: %v", err)
	}
	values := []float64{
		-1.8, -1.4, 0.1,
		0.1, -1.3, 0.7,
		1.6, -1.2, 1.2,
		-1.2, 0.2, 2.1,
		0.7, 0.1, 2.8,
		2.0, 0.4, 3.4,
		-0.4, 1.6, 4.1,
		1.5, 1.7, 5.2,
	}
	eval := newEvaluator(cfg)
	side := 4.2

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		benchmarkPenalty = eval.value(values, side)
	}
}

func BenchmarkMinimizeLBFGS(b *testing.B) {
	cfg, err := parseArgs([]string{"8", "6", "8", "--attempts", "1"})
	if err != nil {
		b.Fatalf("parseArgs returned error: %v", err)
	}
	values := []float64{
		-1.8, -1.4, 0.1,
		0.1, -1.3, 0.7,
		1.6, -1.2, 1.2,
		-1.2, 0.2, 2.1,
		0.7, 0.1, 2.8,
		2.0, 0.4, 3.4,
		-0.4, 1.6, 4.1,
		1.5, 1.7, 5.2,
	}
	side := 4.2

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		objective := newPackingObjective(cfg, side)
		benchmarkOptResult = minimizeLBFGSWithGradient(values, objective.value, objective.gradient, 1e-8)
	}
}

func BenchmarkEvaluatorValueLarge(b *testing.B) {
	cfg, err := parseArgs([]string{"40", "6", "8", "--attempts", "1"})
	if err != nil {
		b.Fatalf("parseArgs returned error: %v", err)
	}
	values := make([]float64, cfg.innerPolygons*3)
	for i := range cfg.innerPolygons {
		values[i*3] = float64(i%8)*1.4 - 4.9
		values[i*3+1] = float64(i/8)*1.4 - 2.8
		values[i*3+2] = float64(i) * 0.37
	}
	eval := newEvaluator(cfg)
	side := 8.0

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		benchmarkPenalty = eval.value(values, side)
	}
}

func bruteForcePenalty(cfg *config, values []float64, side float64) float64 {
	polys := make([]point, cfg.innerPolygons*cfg.innerSides)
	vectors := make([]point, cfg.innerPolygons*cfg.innerSides)
	penalty := 0.0

	for i := range cfg.innerPolygons {
		polygon := polys[i*cfg.innerSides : (i+1)*cfg.innerSides]
		polygonVectors := vectors[i*cfg.innerSides : (i+1)*cfg.innerSides]
		transformPolygon(values[i*3], values[i*3+1], values[i*3+2], cfg.unitPolygonVertices, polygon)
		rotateVectors(values[i*3+2], cfg.unitPolygonVectors, polygonVectors)

		limit := cfg.unitContainerApothem * side
		for _, vertex := range polygon {
			for _, vector := range cfg.unitContainerVectors {
				distance := vertex.x*vector.x + vertex.y*vector.y
				if distance > limit {
					diff := distance - limit
					penalty += diff * diff
				}
			}
		}
	}

	for i := range cfg.innerPolygons {
		polygonI := polys[i*cfg.innerSides : (i+1)*cfg.innerSides]
		vectorsI := vectors[i*cfg.innerSides : (i+1)*cfg.innerSides]
		for j := i + 1; j < cfg.innerPolygons; j++ {
			polygonJ := polys[j*cfg.innerSides : (j+1)*cfg.innerSides]
			vectorsJ := vectors[j*cfg.innerSides : (j+1)*cfg.innerSides]
			collision := true
			minOverlap := 1e20

			for axis := range cfg.innerSides * 2 {
				var axisX, axisY float64
				if axis < cfg.innerSides {
					axisX = vectorsI[axis].x
					axisY = vectorsI[axis].y
				} else {
					vector := vectorsJ[axis-cfg.innerSides]
					axisX = vector.x
					axisY = vector.y
				}

				minI, maxI := bruteProjectPolygon(polygonI, axisX, axisY)
				minJ, maxJ := bruteProjectPolygon(polygonJ, axisX, axisY)
				overlap := min(maxI, maxJ) - max(minI, minJ)
				if overlap <= 0 {
					collision = false
					break
				}
				if overlap < minOverlap {
					minOverlap = overlap
				}
			}

			if collision {
				penalty += minOverlap * minOverlap
			}
		}
	}

	return penalty
}

func bruteProjectPolygon(vertices []point, axisX, axisY float64) (float64, float64) {
	minValue := 1e20
	maxValue := -1e20
	for _, vertex := range vertices {
		d := vertex.x*axisX + vertex.y*axisY
		if d < minValue {
			minValue = d
		}
		if d > maxValue {
			maxValue = d
		}
	}
	return minValue, maxValue
}

func TestPackingObjectiveValue(t *testing.T) {
	cfg, _ := parseArgs([]string{"2", "4", "6", "--attempts", "1"})
	obj := newPackingObjective(cfg, 5.0)
	values := []float64{1, 0, 0, -1, 0, 0.5}
	penalty := obj.value(values)
	if penalty < 0 {
		t.Fatalf("penalty = %g, want non-negative", penalty)
	}

	// Should match evaluator.value directly
	eval := newEvaluator(cfg)
	expected := eval.value(values, 5.0)
	if math.Abs(penalty-expected) > 1e-12 {
		t.Fatalf("packingObjective.value = %g, want %g", penalty, expected)
	}
}

func TestPackingObjectiveGradient(t *testing.T) {
	cfg, _ := parseArgs([]string{"3", "4", "6", "--attempts", "1"})
	obj := newPackingObjective(cfg, 4.0)
	values := []float64{0.5, 0.3, 0.1, -0.4, 0.2, 0.8, 0.1, -0.3, 1.2}
	f0 := obj.value(values)
	gradient := make([]float64, len(values))

	evals := obj.gradient(values, f0, gradient, len(values))
	if evals <= 0 {
		t.Fatalf("evals = %d, want positive", evals)
	}
	for _, g := range gradient {
		if math.IsNaN(g) || math.IsInf(g, 0) {
			t.Fatalf("gradient contains NaN or Inf")
		}
	}
}

func TestSpatialCollisionPenaltyDirect(t *testing.T) {
	cfg, _ := parseArgs([]string{"30", "6", "8", "--attempts", "1"})
	eval := newEvaluator(cfg)
	rng := rand.New(rand.NewSource(99))

	values := make([]float64, cfg.innerPolygons*3)
	for i := range cfg.innerPolygons {
		values[i*3] = rng.Float64()*10 - 5
		values[i*3+1] = rng.Float64()*10 - 5
		values[i*3+2] = rng.Float64() * 2 * math.Pi
	}

	spatialPenalty := eval.spatialCollisionPenalty(values)
	if spatialPenalty < 0 {
		t.Fatalf("spatialCollisionPenalty = %g, want non-negative", spatialPenalty)
	}
}

func TestBuildSpatialGrid(t *testing.T) {
	cfg, _ := parseArgs([]string{"10", "4", "6", "--attempts", "1"})
	eval := newEvaluator(cfg)
	values := make([]float64, cfg.innerPolygons*3)
	for i := range cfg.innerPolygons {
		values[i*3] = float64(i) * 0.5
		values[i*3+1] = float64(i) * 0.3
		values[i*3+2] = 0
	}

	eval.buildSpatialGrid(values)
	if len(eval.cellHeads) == 0 {
		t.Fatal("cellHeads should not be empty after buildSpatialGrid")
	}
	if len(eval.usedCells) == 0 {
		t.Fatal("usedCells should not be empty after buildSpatialGrid")
	}
}

func TestEnsurePairPenaltiesGrowth(t *testing.T) {
	cfg, _ := parseArgs([]string{"5", "4", "6", "--attempts", "1"})
	eval := newEvaluator(cfg)

	// Initially pairPenalties has cap 0
	eval.ensurePairPenalties()
	expected := 5 * 4 / 2 // 10 pairs
	if len(eval.pairPenalties) != expected {
		t.Fatalf("pairPenalties len = %d, want %d", len(eval.pairPenalties), expected)
	}
}

func TestEnsurePairPenaltiesReuse(t *testing.T) {
	cfg, _ := parseArgs([]string{"5", "4", "6", "--attempts", "1"})
	eval := newEvaluator(cfg)

	// First call allocates
	eval.ensurePairPenalties()
	// Second call should reuse
	eval.ensurePairPenalties()
	if len(eval.pairPenalties) != 10 {
		t.Fatalf("pairPenalties len = %d, want 10", len(eval.pairPenalties))
	}
}

func TestFiniteDifferenceGradientSpatialGridPath(t *testing.T) {
	cfg, _ := parseArgs([]string{"100", "4", "6", "--attempts", "1"})
	eval := newEvaluator(cfg)
	values := make([]float64, cfg.innerPolygons*3)
	rng := rand.New(rand.NewSource(7))
	for i := range cfg.innerPolygons {
		values[i*3] = rng.Float64()*10 - 5
		values[i*3+1] = rng.Float64()*10 - 5
		values[i*3+2] = rng.Float64() * 2 * math.Pi
	}

	side := 8.0
	f0 := eval.value(values, side)
	gradient := make([]float64, len(values))

	evals := eval.finiteDifferenceGradient(values, side, f0, gradient, len(values))
	if evals <= 0 {
		t.Fatalf("evals = %d, want positive", evals)
	}
	for _, g := range gradient {
		if math.IsNaN(g) || math.IsInf(g, 0) {
			t.Fatalf("gradient contains NaN or Inf")
		}
	}
}

func TestFiniteDifferenceGradientZeroMaxEvals(t *testing.T) {
	cfg, _ := parseArgs([]string{"3", "4", "6", "--attempts", "1"})
	eval := newEvaluator(cfg)
	values := []float64{0.5, 0.3, 0.1, -0.4, 0.2, 0.8, 0.1, -0.3, 1.2}
	gradient := make([]float64, len(values))

	evals := eval.incrementalFiniteDifferenceGradient(values, 4.0, 1.0, gradient, 0)
	if evals != 0 {
		t.Fatalf("evals = %d, want 0", evals)
	}
	for _, g := range gradient {
		if g != 0 {
			t.Fatalf("gradient should be all zeros, got %g", g)
		}
	}
}

func TestEvaluatorPenaltyOutsideContainer(t *testing.T) {
	cfg, _ := parseArgs([]string{"1", "3", "4", "--attempts", "1"})
	eval := newEvaluator(cfg)
	// Place polygon far outside container
	values := []float64{100, 100, 0}
	penalty := eval.value(values, 1.0)
	if penalty <= 0 {
		t.Fatalf("penalty = %g, want positive for polygon outside container", penalty)
	}
}

func TestEvaluatorNoOverlap(t *testing.T) {
	cfg, _ := parseArgs([]string{"2", "3", "6", "--attempts", "1"})
	eval := newEvaluator(cfg)
	// Place two small polygons far apart with a large container
	values := []float64{-5, 0, 0, 5, 0, 0}
	penalty := eval.value(values, 10.0)
	if penalty != 0 {
		t.Fatalf("penalty = %g, want 0 for non-overlapping polygons in large container", penalty)
	}
}

func TestValueWithPairPenalties(t *testing.T) {
	cfg, _ := parseArgs([]string{"3", "4", "6", "--attempts", "1"})
	eval := newEvaluator(cfg)
	values := []float64{0.5, 0.5, 0, -0.5, 0.5, 1.0, 0, -0.5, 2.0}

	penalty := eval.valueWithPairPenalties(values, 4.0)
	if penalty < 0 {
		t.Fatalf("penalty = %g, want non-negative", penalty)
	}

	// Should have stored individual penalties
	if len(eval.polygonPenalties) != cfg.innerPolygons {
		t.Fatalf("polygonPenalties len = %d, want %d", len(eval.polygonPenalties), cfg.innerPolygons)
	}
	if len(eval.pairPenalties) != cfg.innerPolygons*(cfg.innerPolygons-1)/2 {
		t.Fatalf("pairPenalties len = %d, want %d", len(eval.pairPenalties), cfg.innerPolygons*(cfg.innerPolygons-1)/2)
	}
}

func TestPairPenaltyDistantPolygons(t *testing.T) {
	cfg, _ := parseArgs([]string{"2", "4", "6", "--attempts", "1"})
	eval := newEvaluator(cfg)
	// Place polygons very far apart (> 2 apart, so dx*dx+dy*dy >= 4)
	values := []float64{-10, 0, 0, 10, 0, 0}

	// Need to first compute the polygons via value() to populate polys/vectors
	eval.value(values, 5.0)

	pp := eval.pairPenalty(values, 0, 1)
	if pp != 0 {
		t.Fatalf("pairPenalty for distant polygons = %g, want 0", pp)
	}
}
