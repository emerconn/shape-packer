package main

import (
	"math"
	"math/rand"
	"testing"
)

func TestScaleFloat64s(t *testing.T) {
	result := scaleFloat64s([]float64{1, 2, 3}, 2.5)
	if len(result) != 3 {
		t.Fatalf("len = %d, want 3", len(result))
	}
	expected := []float64{2.5, 5, 7.5}
	for i, v := range result {
		if math.Abs(v-expected[i]) > 1e-12 {
			t.Fatalf("result[%d] = %g, want %g", i, v, expected[i])
		}
	}
}

func TestScaleFloat64sZero(t *testing.T) {
	result := scaleFloat64s([]float64{1, 2, 3}, 0)
	for _, v := range result {
		if v != 0 {
			t.Fatalf("result = %v, want all zeros", result)
		}
	}
}

func TestScaleFloat64sEmpty(t *testing.T) {
	result := scaleFloat64s([]float64{}, 2)
	if len(result) != 0 {
		t.Fatalf("len = %d, want 0", len(result))
	}
}

func TestSubtract(t *testing.T) {
	result := subtract([]float64{5, 3, 1}, []float64{1, 2, 3})
	expected := []float64{4, 1, -2}
	for i, v := range result {
		if math.Abs(v-expected[i]) > 1e-12 {
			t.Fatalf("result[%d] = %g, want %g", i, v, expected[i])
		}
	}
}

func TestDot(t *testing.T) {
	result := dot([]float64{1, 2, 3}, []float64{4, 5, 6})
	expected := 1.0*4 + 2.0*5 + 3.0*6 // 32
	if math.Abs(result-expected) > 1e-12 {
		t.Fatalf("dot = %g, want %g", result, expected)
	}
}

func TestDotOrthogonal(t *testing.T) {
	result := dot([]float64{1, 0}, []float64{0, 1})
	if result != 0 {
		t.Fatalf("dot = %g, want 0", result)
	}
}

func TestDotEmpty(t *testing.T) {
	result := dot([]float64{}, []float64{})
	if result != 0 {
		t.Fatalf("dot = %g, want 0", result)
	}
}

func TestAxpy(t *testing.T) {
	dst := []float64{1, 2, 3}
	axpy(dst, []float64{4, 5, 6}, 2)
	expected := []float64{9, 12, 15}
	for i, v := range dst {
		if math.Abs(v-expected[i]) > 1e-12 {
			t.Fatalf("dst[%d] = %g, want %g", i, v, expected[i])
		}
	}
}

func TestAxpyZero(t *testing.T) {
	dst := []float64{1, 2, 3}
	axpy(dst, []float64{4, 5, 6}, 0)
	expected := []float64{1, 2, 3}
	for i, v := range dst {
		if math.Abs(v-expected[i]) > 1e-12 {
			t.Fatalf("dst[%d] = %g, want %g", i, v, expected[i])
		}
	}
}

func TestAxpyNegative(t *testing.T) {
	dst := []float64{5, 5, 5}
	axpy(dst, []float64{1, 2, 3}, -1)
	expected := []float64{4, 3, 2}
	for i, v := range dst {
		if math.Abs(v-expected[i]) > 1e-12 {
			t.Fatalf("dst[%d] = %g, want %g", i, v, expected[i])
		}
	}
}

func TestMaxAbs(t *testing.T) {
	if v := maxAbs([]float64{-3, 1, 2}); v != 3 {
		t.Fatalf("maxAbs = %g, want 3", v)
	}
	if v := maxAbs([]float64{0.5, -0.1, 0.3}); v != 0.5 {
		t.Fatalf("maxAbs = %g, want 0.5", v)
	}
	if v := maxAbs([]float64{}); v != 0 {
		t.Fatalf("maxAbs = %g, want 0", v)
	}
}

func TestAllFinite(t *testing.T) {
	if !allFinite([]float64{1, 2, 3}) {
		t.Fatal("allFinite([1,2,3]) = false, want true")
	}
	if allFinite([]float64{1, math.NaN(), 3}) {
		t.Fatal("allFinite with NaN = true, want false")
	}
	if allFinite([]float64{1, math.Inf(1), 3}) {
		t.Fatal("allFinite with +Inf = true, want false")
	}
	if allFinite([]float64{1, math.Inf(-1)}) {
		t.Fatal("allFinite with -Inf = true, want false")
	}
	if !allFinite([]float64{}) {
		t.Fatal("allFinite([]) = false, want true")
	}
}

func TestMinimizeLBFGSWithGradientSimpleQuadratic(t *testing.T) {
	// Minimize f(x) = sum(x_i^2) which has minimum at x = 0
	x0 := []float64{1.0, 2.0, 3.0}
	objective := func(x []float64) float64 {
		sum := 0.0
		for _, v := range x {
			sum += v * v
		}
		return sum
	}
	gradFn := func(x []float64, f0 float64, gradient []float64, maxEvals int) int {
		return finiteDifferenceGradient(objective, x, f0, gradient, maxEvals)
	}
	result := minimizeLBFGSWithGradient(x0, objective, gradFn, 1e-10)
	if result.fun > 1e-8 {
		t.Fatalf("fun = %g, want near 0", result.fun)
	}
	for _, v := range result.x {
		if math.Abs(v) > 1e-3 {
			t.Fatalf("x contains %g, want near 0", v)
		}
	}
}

func TestMinimizeLBFGSWithGradientAlreadyAtMinimum(t *testing.T) {
	objective := func(x []float64) float64 { return 0 }
	gradFn := func(x []float64, f0 float64, gradient []float64, maxEvals int) int {
		return finiteDifferenceGradient(objective, x, f0, gradient, maxEvals)
	}
	result := minimizeLBFGSWithGradient([]float64{1, 2, 3}, objective, gradFn, 1e-8)
	if result.fun != 0 {
		t.Fatalf("fun = %g, want 0", result.fun)
	}
	if result.evals != 1 {
		t.Fatalf("evals = %d, want 1", result.evals)
	}
}

func TestMinimizeLBFGSWithGradientRosenbrock2D(t *testing.T) {
	// 2D Rosenbrock: f(x,y) = (1-x)^2 + 100*(y-x^2)^2
	// Minimum at (1,1)
	objective := func(x []float64) float64 {
		dx := 1 - x[0]
		dy := x[1] - x[0]*x[0]
		return dx*dx + 100*dy*dy
	}
	gradFn := func(x []float64, f0 float64, gradient []float64, maxEvals int) int {
		return finiteDifferenceGradient(objective, x, f0, gradient, maxEvals)
	}

	x0 := []float64{-1, 1}
	result := minimizeLBFGSWithGradient(x0, objective, gradFn, 1e-12)
	if result.fun > 0.01 {
		t.Fatalf("fun = %g, want near 0", result.fun)
	}
	if math.Abs(result.x[0]-1) > 0.1 || math.Abs(result.x[1]-1) > 0.1 {
		t.Fatalf("x = %v, want near (1,1)", result.x)
	}
}

func TestLineSearchImprovement(t *testing.T) {
	objective := func(x []float64) float64 { return x[0]*x[0] + x[1]*x[1] }
	x := []float64{1.0, 1.0}
	gradient := []float64{2.0, 2.0}
	direction := []float64{-2.0, -2.0}
	trial := make([]float64, 2)
	bestX := make([]float64, 2)

	fun, evals, ok := lineSearch(objective, x, 2.0, gradient, direction, trial, bestX, 100)
	if !ok {
		t.Fatal("lineSearch should succeed")
	}
	if fun >= 2.0 {
		t.Fatalf("fun = %g, should be less than 2.0", fun)
	}
	if evals <= 0 {
		t.Fatalf("evals = %d, want positive", evals)
	}
}

func TestLineSearchPositiveDerivativeFails(t *testing.T) {
	objective := func(x []float64) float64 { return x[0] * x[0] }
	x := []float64{1.0}
	gradient := []float64{2.0}
	direction := []float64{2.0} // same sign as gradient
	trial := make([]float64, 1)
	bestX := make([]float64, 1)

	_, _, ok := lineSearch(objective, x, 1.0, gradient, direction, trial, bestX, 100)
	if ok {
		t.Fatal("lineSearch should fail with positive derivative")
	}
}

func TestLineSearchZeroMaxEvals(t *testing.T) {
	objective := func(x []float64) float64 { return 0 }
	_, _, ok := lineSearch(objective, []float64{0}, 0, []float64{-1}, []float64{-1}, []float64{0}, []float64{0}, 0)
	if ok {
		t.Fatal("lineSearch should fail with 0 maxEvals")
	}
}

func TestBasinHoppingImproves(t *testing.T) {
	// Simple quadratic, starting near minimum
	objective := func(x []float64) float64 { return x[0]*x[0] + x[1]*x[1] }
	gradFn := func(x []float64, f0 float64, gradient []float64, maxEvals int) int {
		return finiteDifferenceGradient(objective, x, f0, gradient, maxEvals)
	}

	current := optResult{x: []float64{0.1, 0.1}, fun: 0.02}
	rng := rand.New(rand.NewSource(42))

	result := basinHopping(current, objective, gradFn, rng)
	if result.fun > current.fun*10 {
		t.Fatalf("basinHopping fun = %g, should be similar or better than %g", result.fun, current.fun)
	}
}

func TestFiniteDifferenceGradientOptimizer(t *testing.T) {
	objective := func(x []float64) float64 { return x[0]*x[0] + x[1]*x[1] }
	x := []float64{3.0, 4.0}
	f0 := objective(x)
	gradient := make([]float64, 2)

	evals := finiteDifferenceGradient(objective, x, f0, gradient, 100)
	if evals != 2 {
		t.Fatalf("evals = %d, want 2", evals)
	}
	// d/dx(x^2+y^2) at (3,4) = (6,8)
	if math.Abs(gradient[0]-6) > 1e-4 {
		t.Fatalf("gradient[0] = %g, want 6", gradient[0])
	}
	if math.Abs(gradient[1]-8) > 1e-4 {
		t.Fatalf("gradient[1] = %g, want 8", gradient[1])
	}
}

func TestFiniteDifferenceGradientMaxEvalsExhausted(t *testing.T) {
	objective := func(x []float64) float64 { return x[0]*x[0] + x[1]*x[1] + x[2]*x[2] }
	x := []float64{1, 2, 3}
	gradient := make([]float64, 3)

	evals := finiteDifferenceGradient(objective, x, objective(x), gradient, 1)
	if evals != 1 {
		t.Fatalf("evals = %d, want 1", evals)
	}
	// First gradient element should be computed (approx 2*x[0])
	if gradient[0] == 0 {
		t.Fatalf("gradient[0] should be computed")
	}
	// Remaining should be zero since max evals exhausted
	if gradient[1] != 0 || gradient[2] != 0 {
		t.Fatalf("gradient[1:] = %v, want zeros", gradient[1:])
	}
}

func TestFiniteDifferenceGradientInfObjective(t *testing.T) {
	objective := func(x []float64) float64 { return math.Inf(1) }
	x := []float64{1.0}
	gradient := make([]float64, 1)

	finiteDifferenceGradient(objective, x, 0, gradient, 10)
	if gradient[0] != 0 {
		t.Fatalf("gradient[0] = %g, want 0 for Inf objective", gradient[0])
	}
}

func TestLbfgsDirectionEmptyHistory(t *testing.T) {
	direction := []float64{1, 2, 3}
	gradient := []float64{4, 5, 6}
	lbfgsDirection(direction, gradient, nil, nil, nil, make([]float64, 10))

	// With empty history, direction should be -gradient (scaled by 1.0)
	for i, v := range direction {
		if math.Abs(v-(-gradient[i])) > 1e-12 {
			t.Fatalf("direction[%d] = %g, want %g", i, v, -gradient[i])
		}
	}
}

func TestLbfgsDirectionWithHistory(t *testing.T) {
	direction := make([]float64, 2)
	gradient := []float64{1, 1}
	sHistory := [][]float64{{1, 0}}
	yHistory := [][]float64{{0, 1}}
	rhoHistory := []float64{1.0}
	alpha := make([]float64, 10)

	lbfgsDirection(direction, gradient, sHistory, yHistory, rhoHistory, alpha)

	if !allFinite(direction) {
		t.Fatalf("direction = %v, should be finite", direction)
	}
}

func TestMinimizeLBFGSPackingObjective(t *testing.T) {
	cfg, _ := parseArgs([]string{"3", "4", "6", "--attempts", "1"})
	objective := newPackingObjective(cfg, 3.0)
	x0 := []float64{0.5, 0.5, 0.1, -0.5, 0.3, 0.5, 0.1, -0.4, 0.8}

	result := minimizeLBFGSWithGradient(x0, objective.value, objective.gradient, 1e-8)
	if !allFinite(result.x) {
		t.Fatalf("result.x not finite: %v", result.x)
	}
	if math.IsNaN(result.fun) || math.IsInf(result.fun, 0) {
		t.Fatalf("result.fun = %g, should be finite", result.fun)
	}
	if result.iterations == 0 {
		t.Fatal("expected some iterations")
	}
}

func TestSubtractEmpty(t *testing.T) {
	result := subtract([]float64{}, []float64{})
	if len(result) != 0 {
		t.Fatalf("len = %d, want 0", len(result))
	}
}

func TestLineSearchBestEffortImprovement(t *testing.T) {
	// Objective that only improves slightly, not meeting Armijo condition
	// but still returns the best found point
	callCount := 0
	objective := func(x []float64) float64 {
		callCount++
		if callCount <= 1 {
			return 0.99 // slight improvement over f0=1
		}
		return 2.0 // worse after that
	}
	x := []float64{1.0}
	gradient := []float64{-1.0} // descent direction
	direction := []float64{-1.0}
	trial := make([]float64, 1)
	bestX := make([]float64, 1)

	fun, _, ok := lineSearch(objective, x, 1.0, gradient, direction, trial, bestX, 100)
	// Should eventually return improvement even if Armijo isn't met
	if ok && fun >= 1.0 {
		t.Fatalf("fun = %g, should be less than f0=1", fun)
	}
}

func TestMinimizeLBFGSWithGradientHistoryEviction(t *testing.T) {
	// Use a function that requires many iterations to force history eviction
	objective := func(x []float64) float64 {
		sum := 0.0
		for i, v := range x {
			sum += float64(i+1) * v * v
		}
		return sum
	}
	gradFn := func(x []float64, f0 float64, gradient []float64, maxEvals int) int {
		return finiteDifferenceGradient(objective, x, f0, gradient, maxEvals)
	}

	x0 := make([]float64, 20)
	for i := range x0 {
		x0[i] = float64(i + 1)
	}

	result := minimizeLBFGSWithGradient(x0, objective, gradFn, 1e-12)
	if result.fun > 1e-6 {
		t.Fatalf("fun = %g, want near 0", result.fun)
	}
}

func TestMinimizeLBFGSWithGradientResetDirection(t *testing.T) {
	// Test the path where dot(direction, gradient) >= 0, causing direction reset
	// This can happen with non-convex functions
	callCount := 0
	objective := func(x []float64) float64 {
		callCount++
		if callCount%3 == 0 {
			return x[0]*x[0] + 1 // return a higher value to perturb
		}
		return x[0]*x[0] + x[1]*x[1]
	}
	gradFn := func(x []float64, f0 float64, gradient []float64, maxEvals int) int {
		return finiteDifferenceGradient(objective, x, f0, gradient, maxEvals)
	}

	x0 := []float64{5.0, -5.0}
	result := minimizeLBFGSWithGradient(x0, objective, gradFn, 1e-8)
	if !allFinite(result.x) {
		t.Fatalf("result.x not finite: %v", result.x)
	}
}

func TestBasinHoppingRejectsWorse(t *testing.T) {
	// With a high enough temperature and bad seed, basin hopping should still run
	objective := func(x []float64) float64 { return x[0]*x[0] + x[1]*x[1] }
	gradFn := func(x []float64, f0 float64, gradient []float64, maxEvals int) int {
		return finiteDifferenceGradient(objective, x, f0, gradient, maxEvals)
	}

	current := optResult{x: []float64{0.001, 0.001}, fun: 2e-6}
	rng := rand.New(rand.NewSource(1))

	result := basinHopping(current, objective, gradFn, rng)
	// Best should be near minimum
	if result.fun > 1 {
		t.Fatalf("fun = %g, should be small", result.fun)
	}
}
