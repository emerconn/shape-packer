package main

import (
	"math"
	"math/rand"
	"testing"
)

func TestPenaltyIsZeroForSingleCenteredPolygonInsideLargeContainer(t *testing.T) {
	cfg, err := parseArgs(testPolygonArgs(1, 3, 4, "--attempts=1"))
	if err != nil {
		t.Fatalf("parseArgs returned error: %v", err)
	}
	eval := newEvaluator(cfg)
	values := []float64{0, 0, 0}
	if penalty := eval.value(values, 3); penalty != 0 {
		t.Fatalf("penalty = %g, want 0", penalty)
	}
}

func TestPenaltyIsZeroForSingleCenteredCircleInsideLargeCircle(t *testing.T) {
	cfg, err := parseArgs(testCircleInCircleArgs(1, "--attempts=1"))
	if err != nil {
		t.Fatalf("parseArgs returned error: %v", err)
	}
	eval := newEvaluator(cfg)
	values := []float64{0, 0}
	if penalty := eval.value(values, 3); penalty != 0 {
		t.Fatalf("penalty = %g, want 0", penalty)
	}
}

func TestPenaltyIsPositiveForOverlappingPolygons(t *testing.T) {
	cfg, err := parseArgs(testPolygonArgs(2, 4, 4, "--attempts=1"))
	if err != nil {
		t.Fatalf("parseArgs returned error: %v", err)
	}
	eval := newEvaluator(cfg)
	values := []float64{0, 0, 0, 0, 0, 0}
	if penalty := eval.value(values, 5); penalty <= 0 {
		t.Fatalf("penalty = %g, want positive", penalty)
	}
}

func TestPenaltyIsPositiveForOverlappingCircles(t *testing.T) {
	cfg, err := parseArgs(testCircleInCircleArgs(2, "--attempts=1"))
	if err != nil {
		t.Fatalf("parseArgs returned error: %v", err)
	}
	eval := newEvaluator(cfg)
	values := []float64{0, 0, 0, 0}
	if penalty := eval.value(values, 5); penalty <= 0 {
		t.Fatalf("penalty = %g, want positive", penalty)
	}
}

func TestOptimizedPenaltyMatchesBruteForceReference(t *testing.T) {
	cases := [][]string{
		testPolygonArgs(3, 3, 4, "--attempts=1"),
		testPolygonArgs(4, 5, 6, "--attempts=1"),
		testPolygonArgs(5, 6, 8, "--attempts=1"),
	}
	rng := rand.New(rand.NewSource(1))

	for _, args := range cases {
		cfg, err := parseArgs(args)
		if err != nil {
			t.Fatalf("parseArgs(%v) returned error: %v", args, err)
		}
		eval := newEvaluator(cfg)

		for trial := 0; trial < 30; trial++ {
			values := make([]float64, cfg.innerCount*cfg.paramsPerShape)
			for i := range cfg.innerCount {
				values[i*cfg.paramsPerShape] = rng.Float64()*6 - 3
				values[i*cfg.paramsPerShape+1] = rng.Float64()*6 - 3
				if cfg.innerIsPolygon() {
					values[i*cfg.paramsPerShape+2] = rng.Float64() * 2 * math.Pi
				}
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

func TestOptimizedPenaltyMatchesBruteForceCircleInCircle(t *testing.T) {
	cfg, err := parseArgs(testCircleInCircleArgs(4, "--attempts=1"))
	if err != nil {
		t.Fatalf("parseArgs returned error: %v", err)
	}
	eval := newEvaluator(cfg)
	rng := rand.New(rand.NewSource(3))

	for trial := 0; trial < 30; trial++ {
		values := make([]float64, cfg.innerCount*cfg.paramsPerShape)
		for i := range cfg.innerCount {
			values[i*cfg.paramsPerShape] = rng.Float64()*6 - 3
			values[i*cfg.paramsPerShape+1] = rng.Float64()*6 - 3
		}

		side := 1.5 + rng.Float64()*4
		got := eval.value(values, side)
		want := bruteForcePenalty(cfg, values, side)
		diff := math.Abs(got - want)
		tolerance := 1e-9 * (1 + max(math.Abs(got), math.Abs(want)))
		if diff > tolerance {
			t.Fatalf("penalty mismatch trial %d: got %g, want %g, diff %g", trial, got, want, diff)
		}
	}
}

func TestOptimizedPenaltyMatchesBruteForceCircleInPolygon(t *testing.T) {
	cfg, err := parseArgs(testCircleInPolygonArgs(3, 6, "--attempts=1"))
	if err != nil {
		t.Fatalf("parseArgs returned error: %v", err)
	}
	eval := newEvaluator(cfg)
	rng := rand.New(rand.NewSource(4))

	for trial := 0; trial < 30; trial++ {
		values := make([]float64, cfg.innerCount*cfg.paramsPerShape)
		for i := range cfg.innerCount {
			values[i*cfg.paramsPerShape] = rng.Float64()*6 - 3
			values[i*cfg.paramsPerShape+1] = rng.Float64()*6 - 3
		}

		side := 1.5 + rng.Float64()*4
		got := eval.value(values, side)
		want := bruteForcePenalty(cfg, values, side)
		diff := math.Abs(got - want)
		tolerance := 1e-9 * (1 + max(math.Abs(got), math.Abs(want)))
		if diff > tolerance {
			t.Fatalf("penalty mismatch trial %d: got %g, want %g, diff %g", trial, got, want, diff)
		}
	}
}

func TestOptimizedPenaltyMatchesBruteForcePolygonInCircle(t *testing.T) {
	cfg, err := parseArgs(testPolygonInCircleArgs(3, 4, "--attempts=1"))
	if err != nil {
		t.Fatalf("parseArgs returned error: %v", err)
	}
	eval := newEvaluator(cfg)
	rng := rand.New(rand.NewSource(5))

	for trial := 0; trial < 30; trial++ {
		values := make([]float64, cfg.innerCount*cfg.paramsPerShape)
		for i := range cfg.innerCount {
			values[i*cfg.paramsPerShape] = rng.Float64()*6 - 3
			values[i*cfg.paramsPerShape+1] = rng.Float64()*6 - 3
			values[i*cfg.paramsPerShape+2] = rng.Float64() * 2 * math.Pi
		}

		side := 1.5 + rng.Float64()*4
		got := eval.value(values, side)
		want := bruteForcePenalty(cfg, values, side)
		diff := math.Abs(got - want)
		tolerance := 1e-9 * (1 + max(math.Abs(got), math.Abs(want)))
		if diff > tolerance {
			t.Fatalf("penalty mismatch trial %d: got %g, want %g, diff %g", trial, got, want, diff)
		}
	}
}

func TestSpatialPenaltyMatchesBruteForceReference(t *testing.T) {
	cfg, err := parseArgs(testPolygonArgs(30, 6, 8, "--attempts=1"))
	if err != nil {
		t.Fatalf("parseArgs returned error: %v", err)
	}
	eval := newEvaluator(cfg)
	rng := rand.New(rand.NewSource(2))

	for trial := 0; trial < 10; trial++ {
		values := make([]float64, cfg.innerCount*cfg.paramsPerShape)
		for i := range cfg.innerCount {
			values[i*cfg.paramsPerShape] = rng.Float64()*10 - 5
			values[i*cfg.paramsPerShape+1] = rng.Float64()*10 - 5
			values[i*cfg.paramsPerShape+2] = rng.Float64() * 2 * math.Pi
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
	cfg, err := parseArgs(testPolygonArgs(6, 5, 7, "--attempts=1"))
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

func TestIncrementalGradientCircleInCircle(t *testing.T) {
	cfg, err := parseArgs(testCircleInCircleArgs(4, "--attempts=1"))
	if err != nil {
		t.Fatalf("parseArgs returned error: %v", err)
	}
	values := []float64{-1.2, -0.9, 0.2, -0.8, 1.1, -0.6, -0.7, 0.4}
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
	cfg, err := parseArgs(testPolygonArgs(8, 6, 8, "--attempts=1"))
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
	cfg, err := parseArgs(testPolygonArgs(8, 6, 8, "--attempts=1"))
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
	cfg, err := parseArgs(testPolygonArgs(40, 6, 8, "--attempts=1"))
	if err != nil {
		b.Fatalf("parseArgs returned error: %v", err)
	}
	values := make([]float64, cfg.innerCount*cfg.paramsPerShape)
	for i := range cfg.innerCount {
		values[i*cfg.paramsPerShape] = float64(i%8)*1.4 - 4.9
		values[i*cfg.paramsPerShape+1] = float64(i/8)*1.4 - 2.8
		values[i*cfg.paramsPerShape+2] = float64(i) * 0.37
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
	penalty := 0.0

	if cfg.innerIsPolygon() {
		polys := make([]point, cfg.innerCount*cfg.innerSides)
		vectors := make([]point, cfg.innerCount*cfg.innerSides)

		for i := range cfg.innerCount {
			polygon := polys[i*cfg.innerSides : (i+1)*cfg.innerSides]
			polygonVectors := vectors[i*cfg.innerSides : (i+1)*cfg.innerSides]
			vb := i * cfg.paramsPerShape
			transformPolygon(values[vb], values[vb+1], values[vb+2], cfg.unitPolygonVertices, polygon)
			rotateVectors(values[vb+2], cfg.unitPolygonVectors, polygonVectors)

			if cfg.outerIsPolygon() {
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
			} else {
				penalty += polygonInCirclePenalty(polygon, side)
			}
		}

		for i := range cfg.innerCount {
			polygonI := polys[i*cfg.innerSides : (i+1)*cfg.innerSides]
			vectorsI := vectors[i*cfg.innerSides : (i+1)*cfg.innerSides]
			for j := i + 1; j < cfg.innerCount; j++ {
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
	} else {
		for i := range cfg.innerCount {
			vb := i * cfg.paramsPerShape
			cx, cy := values[vb], values[vb+1]
			if cfg.outerIsPolygon() {
				penalty += circleInPolygonPenalty(cx, cy, 1.0, cfg.unitContainerVectors, cfg.unitContainerApothem*side)
			} else {
				penalty += circleInCirclePenalty(cx, cy, 1.0, side)
			}
		}

		for i := range cfg.innerCount {
			vi := i * cfg.paramsPerShape
			for j := i + 1; j < cfg.innerCount; j++ {
				vj := j * cfg.paramsPerShape
				dx := values[vi] - values[vj]
				dy := values[vi+1] - values[vj+1]
				dist := math.Sqrt(dx*dx + dy*dy)
				if dist < 2 {
					overlap := 2 - dist
					penalty += overlap * overlap
				}
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
	cfg, _ := parseArgs(testPolygonArgs(2, 4, 6, "--attempts=1"))
	obj := newPackingObjective(cfg, 5.0)
	values := []float64{1, 0, 0, -1, 0, 0.5}
	penalty := obj.value(values)
	if penalty < 0 {
		t.Fatalf("penalty = %g, want non-negative", penalty)
	}

	eval := newEvaluator(cfg)
	expected := eval.value(values, 5.0)
	if math.Abs(penalty-expected) > 1e-12 {
		t.Fatalf("packingObjective.value = %g, want %g", penalty, expected)
	}
}

func TestPackingObjectiveGradient(t *testing.T) {
	cfg, _ := parseArgs(testPolygonArgs(3, 4, 6, "--attempts=1"))
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
	cfg, _ := parseArgs(testPolygonArgs(30, 6, 8, "--attempts=1"))
	eval := newEvaluator(cfg)
	rng := rand.New(rand.NewSource(99))

	values := make([]float64, cfg.innerCount*cfg.paramsPerShape)
	for i := range cfg.innerCount {
		values[i*cfg.paramsPerShape] = rng.Float64()*10 - 5
		values[i*cfg.paramsPerShape+1] = rng.Float64()*10 - 5
		values[i*cfg.paramsPerShape+2] = rng.Float64() * 2 * math.Pi
	}

	spatialPenalty := eval.spatialCollisionPenalty(values)
	if spatialPenalty < 0 {
		t.Fatalf("spatialCollisionPenalty = %g, want non-negative", spatialPenalty)
	}
}

func TestBuildSpatialGrid(t *testing.T) {
	cfg, _ := parseArgs(testPolygonArgs(10, 4, 6, "--attempts=1"))
	eval := newEvaluator(cfg)
	values := make([]float64, cfg.innerCount*cfg.paramsPerShape)
	for i := range cfg.innerCount {
		values[i*cfg.paramsPerShape] = float64(i) * 0.5
		values[i*cfg.paramsPerShape+1] = float64(i) * 0.3
		values[i*cfg.paramsPerShape+2] = 0
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
	cfg, _ := parseArgs(testPolygonArgs(5, 4, 6, "--attempts=1"))
	eval := newEvaluator(cfg)

	eval.ensurePairPenalties()
	expected := 5 * 4 / 2
	if len(eval.pairPenalties) != expected {
		t.Fatalf("pairPenalties len = %d, want %d", len(eval.pairPenalties), expected)
	}
}

func TestEnsurePairPenaltiesReuse(t *testing.T) {
	cfg, _ := parseArgs(testPolygonArgs(5, 4, 6, "--attempts=1"))
	eval := newEvaluator(cfg)

	eval.ensurePairPenalties()
	eval.ensurePairPenalties()
	if len(eval.pairPenalties) != 10 {
		t.Fatalf("pairPenalties len = %d, want 10", len(eval.pairPenalties))
	}
}

func TestFiniteDifferenceGradientSpatialGridPath(t *testing.T) {
	cfg, _ := parseArgs(testPolygonArgs(100, 4, 6, "--attempts=1"))
	eval := newEvaluator(cfg)
	values := make([]float64, cfg.innerCount*cfg.paramsPerShape)
	rng := rand.New(rand.NewSource(7))
	for i := range cfg.innerCount {
		values[i*cfg.paramsPerShape] = rng.Float64()*10 - 5
		values[i*cfg.paramsPerShape+1] = rng.Float64()*10 - 5
		values[i*cfg.paramsPerShape+2] = rng.Float64() * 2 * math.Pi
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
	cfg, _ := parseArgs(testPolygonArgs(3, 4, 6, "--attempts=1"))
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
	cfg, _ := parseArgs(testPolygonArgs(1, 3, 4, "--attempts=1"))
	eval := newEvaluator(cfg)
	values := []float64{100, 100, 0}
	penalty := eval.value(values, 1.0)
	if penalty <= 0 {
		t.Fatalf("penalty = %g, want positive for polygon outside container", penalty)
	}
}

func TestEvaluatorPenaltyCircleOutsideCircleContainer(t *testing.T) {
	cfg, _ := parseArgs(testCircleInCircleArgs(1, "--attempts=1"))
	eval := newEvaluator(cfg)
	values := []float64{100, 100}
	penalty := eval.value(values, 1.0)
	if penalty <= 0 {
		t.Fatalf("penalty = %g, want positive for circle outside container", penalty)
	}
}

func TestEvaluatorNoOverlap(t *testing.T) {
	cfg, _ := parseArgs(testPolygonArgs(2, 3, 6, "--attempts=1"))
	eval := newEvaluator(cfg)
	values := []float64{-5, 0, 0, 5, 0, 0}
	penalty := eval.value(values, 10.0)
	if penalty != 0 {
		t.Fatalf("penalty = %g, want 0 for non-overlapping polygons in large container", penalty)
	}
}

func TestEvaluatorNoOverlapCircles(t *testing.T) {
	cfg, _ := parseArgs(testCircleInCircleArgs(2, "--attempts=1"))
	eval := newEvaluator(cfg)
	values := []float64{-5, 0, 5, 0}
	penalty := eval.value(values, 10.0)
	if penalty != 0 {
		t.Fatalf("penalty = %g, want 0 for non-overlapping circles in large container", penalty)
	}
}

func TestValueWithPairPenalties(t *testing.T) {
	cfg, _ := parseArgs(testPolygonArgs(3, 4, 6, "--attempts=1"))
	eval := newEvaluator(cfg)
	values := []float64{0.5, 0.5, 0, -0.5, 0.5, 1.0, 0, -0.5, 2.0}

	penalty := eval.valueWithPairPenalties(values, 4.0)
	if penalty < 0 {
		t.Fatalf("penalty = %g, want non-negative", penalty)
	}

	if len(eval.polygonPenalties) != cfg.innerCount {
		t.Fatalf("polygonPenalties len = %d, want %d", len(eval.polygonPenalties), cfg.innerCount)
	}
	if len(eval.pairPenalties) != cfg.innerCount*(cfg.innerCount-1)/2 {
		t.Fatalf("pairPenalties len = %d, want %d", len(eval.pairPenalties), cfg.innerCount*(cfg.innerCount-1)/2)
	}
}

func TestPairPenaltyDistantPolygons(t *testing.T) {
	cfg, _ := parseArgs(testPolygonArgs(2, 4, 6, "--attempts=1"))
	eval := newEvaluator(cfg)
	values := []float64{-10, 0, 0, 10, 0, 0}

	eval.value(values, 5.0)

	pp := eval.pairPenalty(values, 0, 1)
	if pp != 0 {
		t.Fatalf("pairPenalty for distant polygons = %g, want 0", pp)
	}
}

func TestPairPenaltyDistantCircles(t *testing.T) {
	cfg, _ := parseArgs(testCircleInCircleArgs(2, "--attempts=1"))
	eval := newEvaluator(cfg)
	values := []float64{-10, 0, 10, 0}

	pp := eval.pairPenalty(values, 0, 1)
	if pp != 0 {
		t.Fatalf("pairPenalty for distant circles = %g, want 0", pp)
	}
}
