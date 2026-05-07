package main

import (
	"math"
	"math/rand"
	"slices"
)

const (
	lbfgsHistorySize       = 10
	lbfgsMaxIterations     = 1000
	lbfgsMaxFunctionEvals  = 15000
	lbfgsGradientEps       = 1e-8
	lbfgsGradientTolerance = 1e-5

	basinHoppingIterations = 50
	basinHoppingTemp       = 0.1
	basinHoppingStepSize   = 0.1
)

type optResult struct {
	x          []float64
	fun        float64
	iterations int
	evals      int
}

type gradientFunc func(x []float64, f0 float64, gradient []float64, maxEvals int) int

func minimizeLBFGSWithGradient(x0 []float64, objective func([]float64) float64, gradientEval gradientFunc, tol float64) optResult {
	n := len(x0)
	x := slices.Clone(x0)
	fun := objective(x)
	evals := 1
	if fun == 0 {
		return optResult{x: x, fun: fun, evals: evals}
	}

	gradient := make([]float64, n)
	newGradient := make([]float64, n)
	direction := make([]float64, n)
	trial := make([]float64, n)
	newX := make([]float64, n)
	alphaHistory := make([]float64, lbfgsHistorySize)
	gradientEvals := gradientEval(x, fun, gradient, lbfgsMaxFunctionEvals-evals)
	evals += gradientEvals

	sHistory := make([][]float64, 0, lbfgsHistorySize)
	yHistory := make([][]float64, 0, lbfgsHistorySize)
	rhoHistory := make([]float64, 0, lbfgsHistorySize)
	iterations := 0

	for iterations < lbfgsMaxIterations && evals < lbfgsMaxFunctionEvals {
		if maxAbs(gradient) <= lbfgsGradientTolerance {
			break
		}

		lbfgsDirection(direction, gradient, sHistory, yHistory, rhoHistory, alphaHistory)
		if dot(direction, gradient) >= 0 || !allFinite(direction) {
			for i := range direction {
				direction[i] = -gradient[i]
			}
		}

		newFun, lineEvals, ok := lineSearch(objective, x, fun, gradient, direction, trial, newX, lbfgsMaxFunctionEvals-evals)
		evals += lineEvals
		if !ok {
			break
		}

		gradientEvals = gradientEval(newX, newFun, newGradient, lbfgsMaxFunctionEvals-evals)
		evals += gradientEvals

		step := subtract(newX, x)
		gradientDelta := subtract(newGradient, gradient)
		ys := dot(gradientDelta, step)
		if ys > 1e-12 && isFinite(ys) {
			if len(sHistory) == lbfgsHistorySize {
				copy(sHistory, sHistory[1:])
				copy(yHistory, yHistory[1:])
				copy(rhoHistory, rhoHistory[1:])
				sHistory = sHistory[:lbfgsHistorySize-1]
				yHistory = yHistory[:lbfgsHistorySize-1]
				rhoHistory = rhoHistory[:lbfgsHistorySize-1]
			}
			sHistory = append(sHistory, step)
			yHistory = append(yHistory, gradientDelta)
			rhoHistory = append(rhoHistory, 1/ys)
		}

		relativeReduction := math.Abs(fun-newFun) / max(1, max(math.Abs(fun), math.Abs(newFun)))
		x, newX = newX, x
		fun = newFun
		gradient, newGradient = newGradient, gradient
		iterations++
		if relativeReduction <= tol {
			break
		}
	}

	return optResult{x: x, fun: fun, iterations: iterations, evals: evals}
}

func finiteDifferenceGradient(objective func([]float64) float64, x []float64, f0 float64, gradient []float64, maxEvals int) int {
	evals := 0
	for i := range x {
		if evals >= maxEvals {
			for j := i; j < len(x); j++ {
				gradient[j] = 0
			}
			break
		}
		original := x[i]
		x[i] = original + lbfgsGradientEps
		f1 := objective(x)
		x[i] = original
		if isFinite(f1) {
			gradient[i] = (f1 - f0) / lbfgsGradientEps
		} else {
			gradient[i] = 0
		}
		evals++
	}
	return evals
}

func lbfgsDirection(direction, gradient []float64, sHistory, yHistory [][]float64, rhoHistory, alpha []float64) {
	copy(direction, gradient)
	for i := len(sHistory) - 1; i >= 0; i-- {
		alpha[i] = rhoHistory[i] * dot(sHistory[i], direction)
		axpy(direction, yHistory[i], -alpha[i])
	}

	scale := 1.0
	if len(sHistory) > 0 {
		lastS := sHistory[len(sHistory)-1]
		lastY := yHistory[len(yHistory)-1]
		yy := dot(lastY, lastY)
		if yy > 0 {
			scale = dot(lastS, lastY) / yy
		}
	}

	for i := range direction {
		direction[i] *= scale
	}
	for i := range sHistory {
		beta := rhoHistory[i] * dot(yHistory[i], direction)
		axpy(direction, sHistory[i], alpha[i]-beta)
	}
	for i := range direction {
		direction[i] = -direction[i]
	}
}

func lineSearch(objective func([]float64) float64, x []float64, f0 float64, gradient []float64, direction []float64, trial []float64, bestX []float64, maxEvals int) (float64, int, bool) {
	derivative := dot(gradient, direction)
	if derivative >= 0 || !isFinite(derivative) || maxEvals <= 0 {
		return 0, 0, false
	}

	const armijo = 1e-4
	step := 1.0
	evals := 0
	bestFun := f0
	improved := false

	for evals < maxEvals && step > 1e-20 {
		for i := range x {
			trial[i] = x[i] + step*direction[i]
		}
		trialFun := objective(trial)
		evals++
		if isFinite(trialFun) {
			if trialFun < bestFun {
				copy(bestX, trial)
				bestFun = trialFun
				improved = true
			}
			if trialFun <= f0+armijo*step*derivative {
				copy(bestX, trial)
				return trialFun, evals, true
			}
		}
		step *= 0.5
	}

	if improved {
		return bestFun, evals, true
	}
	return 0, evals, false
}

func basinHopping(current optResult, objective func([]float64) float64, gradientEval gradientFunc, rng *rand.Rand) optResult {
	best := optResult{
		x:          slices.Clone(current.x),
		fun:        current.fun,
		iterations: current.iterations,
		evals:      current.evals,
	}

	for range basinHoppingIterations {
		trial := slices.Clone(current.x)
		for j := range trial {
			trial[j] += (rng.Float64()*2 - 1) * basinHoppingStepSize
		}

		minimized := minimizeLBFGSWithGradient(trial, objective, gradientEval, 1e-8)
		delta := minimized.fun - current.fun
		if delta < 0 || rng.Float64() < math.Exp(-delta/basinHoppingTemp) {
			current = minimized
			if current.fun < best.fun {
				best = optResult{
					x:          slices.Clone(current.x),
					fun:        current.fun,
					iterations: current.iterations,
					evals:      current.evals,
				}
			}
		}
	}

	return best
}

func scaleFloat64s(values []float64, multiplier float64) []float64 {
	out := make([]float64, len(values))
	for i, v := range values {
		out[i] = v * multiplier
	}
	return out
}

func subtract(a, b []float64) []float64 {
	out := make([]float64, len(a))
	for i := range a {
		out[i] = a[i] - b[i]
	}
	return out
}

func dot(a, b []float64) float64 {
	sum := 0.0
	for i := range a {
		sum += a[i] * b[i]
	}
	return sum
}

func axpy(dst []float64, x []float64, alpha float64) {
	for i := range dst {
		dst[i] += alpha * x[i]
	}
}

func maxAbs(values []float64) float64 {
	m := 0.0
	for _, v := range values {
		m = max(m, math.Abs(v))
	}
	return m
}

func allFinite(values []float64) bool {
	for _, v := range values {
		if !isFinite(v) {
			return false
		}
	}
	return true
}

func isFinite(v float64) bool {
	return !math.IsNaN(v) && !math.IsInf(v, 0)
}
