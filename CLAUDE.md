# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build & Run Commands

```bash
go run . [n] [nsi] [nsc]                          # Run with positional args
go build -o polygon-packer .                       # Build binary
go test -v ./...                                   # Run all tests
go test -run TestParseArgs -v ./...                # Run single test
go test -bench=BenchmarkEvaluatorValue -benchmem -count=3  # Benchmarks
```

Arguments: `n` = number of inner polygons, `nsi` = inner polygon sides, `nsc` = container polygon sides. Flags: `--attempts`, `--tolerance`, `--finalstep`, `--cpuprofile`.

## Architecture

Go application implementing a 2D polygon bin-packing optimizer. All code is in `package main`, split across files by concern:

- `main.go` — entry point, config, CLI, pipeline orchestration
- `geometry.go` — point type, polygon transforms, SAT projection helpers
- `evaluator.go` — collision detection, spatial grid, gradient computation
- `optimizer.go` — LBFGS, basin hopping, line search, vector math
- `plot.go` — PNG rendering, drawing, bitmap font

**Optimization pipeline**: `main()` → `runAttempts()` → `repetition()` per seed. Each repetition iteratively shrinks the container, running LBFGS minimization at each size with basin hopping to escape local minima.

**Key types**:

- `config` — holds all parameters and precomputed polygon geometry
- `evaluator` — computes the penalty (objective) for a given set of polygon placements, owns the spatial grid
- `packingObjective` — wraps evaluator for the optimizer, implementing `value()` and `gradient()`
- `gridCell` — spatial hashing for O(n) collision detection instead of O(n²)

**Optimization algorithms** (all implemented from scratch, no external libs):

- `minimizeLBFGS` / `minimizeLBFGSWithGradient` — quasi-Newton optimizer with custom line search
- `basinHopping` — perturbation-based global optimization
- `incrementalFiniteDifferenceGradient` — efficient gradient computation that reuses unchanged polygon pairs

**Geometry**: SAT (Separating Axis Theorem) based collision via `pairPenalty()`, which projects polygons onto axes and computes interval overlap. `transformPolygonAndVectors()` places polygons by (x, y, angle).

**Output**: `savePlot()` renders the best packing as PNG files at multiple scales, alongside a text summary with coordinates.

## Deployment

- Docker images at `ghcr.io/emerconn/polygon-packer` (multi-arch, slim + debug variants)
- `deploy.sh` creates Google Cloud Run jobs for batch processing across polygon counts
- CI/CD via `.github/workflows/test-build-release.yml` — tests, cross-compile, container build, and GitHub release
