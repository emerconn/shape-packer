package main

import (
	"math"
	"math/rand"
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
			for i := 0; i < cfg.innerPolygons; i++ {
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
		for i := 0; i < cfg.innerPolygons; i++ {
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

func TestPlotRotationAlignsCommonContainerEdges(t *testing.T) {
	cfg, err := parseArgs([]string{"1", "3", "3", "--attempts", "1"})
	if err != nil {
		t.Fatalf("parseArgs returned error: %v", err)
	}
	triangle := rotatedContainer(cfg)
	if !hasHorizontalEdge(triangle) {
		t.Fatalf("rotated triangle container does not have a horizontal side: %#v", triangle)
	}

	cfg, err = parseArgs([]string{"1", "3", "4", "--attempts", "1"})
	if err != nil {
		t.Fatalf("parseArgs returned error: %v", err)
	}
	square := rotatedContainer(cfg)
	for i := range square {
		a := square[i]
		b := square[(i+1)%len(square)]
		if math.Abs(a.x-b.x) > 1e-12 && math.Abs(a.y-b.y) > 1e-12 {
			t.Fatalf("rotated square edge %d is not horizontal or vertical: %#v -> %#v", i, a, b)
		}
	}
}

func rotatedContainer(cfg *config) []point {
	cosAngle, sinAngle := plotRotation(cfg.containerSides)
	vertices := make([]point, cfg.containerSides)
	for i, vertex := range cfg.unitContainerVertices {
		vertices[i] = rotatePoint(vertex, cosAngle, sinAngle)
	}
	return vertices
}

func hasHorizontalEdge(vertices []point) bool {
	for i := range vertices {
		if math.Abs(vertices[i].y-vertices[(i+1)%len(vertices)].y) <= 1e-12 {
			return true
		}
	}
	return false
}

func bruteForcePenalty(cfg *config, values []float64, side float64) float64 {
	polys := make([]point, cfg.innerPolygons*cfg.innerSides)
	vectors := make([]point, cfg.innerPolygons*cfg.innerSides)
	penalty := 0.0

	for i := 0; i < cfg.innerPolygons; i++ {
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

	for i := 0; i < cfg.innerPolygons; i++ {
		polygonI := polys[i*cfg.innerSides : (i+1)*cfg.innerSides]
		vectorsI := vectors[i*cfg.innerSides : (i+1)*cfg.innerSides]
		for j := i + 1; j < cfg.innerPolygons; j++ {
			polygonJ := polys[j*cfg.innerSides : (j+1)*cfg.innerSides]
			vectorsJ := vectors[j*cfg.innerSides : (j+1)*cfg.innerSides]
			collision := true
			minOverlap := 1e20

			for axis := 0; axis < cfg.innerSides*2; axis++ {
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
		dot := vertex.x*axisX + vertex.y*axisY
		if dot < minValue {
			minValue = dot
		}
		if dot > maxValue {
			maxValue = dot
		}
	}
	return minValue, maxValue
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
	for i := 0; i < cfg.innerPolygons; i++ {
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
