# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build & Run Commands

```bash
go run . --inner-count=3 --inner-sides=3 --outer-sides=3   # Run with flags
go build -o polygon-packer .                                # Build binary
go test -v ./...                                            # Run all tests
go test -run TestParseArgs -v ./...                          # Run single test
go test -bench=BenchmarkEvaluatorValue -benchmem -count=3   # Benchmarks
```

Flags: `--inner-count` (required), `--inner-sides` (required, number or `c` for circle), `--outer-sides` (required, number or `c` for circle), `--attempts`, `--tolerance`, `--finalstep`, `--cpuprofile`.

## Architecture

Go application implementing a 2D shape bin-packing optimizer. Supports four inner/outer combinations: polygon-in-polygon, circle-in-circle, polygon-in-circle, circle-in-polygon. All code is in `package main`, split across files by concern:

- `main.go` — entry point, config, CLI, pipeline orchestration
- `geometry.go` — point type, polygon transforms, circle geometry, SAT projection helpers
- `evaluator.go` — collision detection (SAT for polygons, distance-based for circles), spatial grid, gradient computation
- `optimizer.go` — LBFGS, basin hopping, line search, vector math
- `plot.go` — PNG rendering, drawing, bitmap font

**Optimization pipeline**: `main()` → `runAttempts()` → `repetition()` per seed. Each repetition iteratively shrinks the container, running LBFGS minimization at each size with basin hopping to escape local minima.

**Key types**:

- `config` — holds all parameters, inner/outer type (`"polygon"` or `"circle"`), and precomputed geometry
- `evaluator` — computes the penalty (objective) for a given set of shape placements, owns the spatial grid
- `packingObjective` — wraps evaluator for the optimizer, implementing `value()` and `gradient()`
- `gridCell` — spatial hashing for O(n) collision detection instead of O(n²)

**Shape parameterization**: Polygon inner shapes use 3 parameters per shape (x, y, angle). Circle inner shapes use 2 parameters (x, y). This is tracked by `config.paramsPerShape`.

**Container penalties** (4 cases, dispatched by `computeContainerPenalty`):
- Polygon in polygon: SAT-based via `transformPolygonAndVectors`
- Circle in polygon: project center + radius onto container face normals via `circleInPolygonPenalty`
- Polygon in circle: check each vertex distance via `polygonInCirclePenalty`
- Circle in circle: distance check via `circleInCirclePenalty`

**Pair penalties** (2 cases, dispatched by `pairPenalty`):
- Polygon-polygon: SAT via `polygonPairPenalty`
- Circle-circle: distance-based via `circlePairPenalty`

**Optimization algorithms** (all implemented from scratch, no external libs):

- `minimizeLBFGS` / `minimizeLBFGSWithGradient` — quasi-Newton optimizer with custom line search
- `basinHopping` — perturbation-based global optimization
- `incrementalFiniteDifferenceGradient` — efficient gradient computation that reuses unchanged shape pairs

**Output**: `savePlot()` renders the best packing as PNG files at multiple scales. Reports container side length (polygon container) or container radius (circle container), normalized to inner shape side length = 1.

## Deployment

- Docker images at `ghcr.io/emerconn/polygon-packer` (multi-arch, slim + debug variants)
- `deploy.sh` creates Google Cloud Run jobs for batch processing across polygon counts
- CI/CD via `.github/workflows/test-build-release.yml` — tests, cross-compile, container build, and GitHub release
