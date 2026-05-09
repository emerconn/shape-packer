# emerconn's shape packer

This program solves the 2D bin packing problem for shapes inside containers. It supports **polygons and circles** in any combination — polygons in polygons, circles in circles, polygons in circles, and circles in polygons. It was the tool used to find all the optimal packings under the name "Emerson Connelly" on [Erich's Packing Center](https://erich-friedman.github.io/packing/).

This is a fork of [Flamethrower's polygon-packer](https://github.com/Flamethr0wer/polygon-packer). All credit goes to him for the crazy math he did.

My fork is an optimized version in Go, roughly 40x to 45x faster than the original Python version. Both tests ran on the same PC (AMD Ryzen 9 5950X) under the same conditions.

Optimized Go version:

```bash
❯ time ./polygon-packer --inner-count=3 --inner-sides=3 --outer-sides=3
Attempt 0
...
Attempt 999
Final side length: 1.999935429205552
./polygon-packer  8.55s user 1.90s system 950% cpu 1.099 total
```

Original Python version:

```bash
❯ time python3 polygon_packer.py 3 3 3
Attempt 0
...
Attempt 999
Final side length: 1.9999356632378391
python3 polygon_packer.py 3 3 3  1414.84s user 50.07s system 3118% cpu 46.975 total
```

## How to use

Build the binary:

```bash
go build -o polygon-packer .
```

Run it:

```bash
./polygon-packer --inner-count=N --inner-sides=S --outer-sides=S
```

### Required flags

| Flag | Description |
|------|-------------|
| `--inner-count N` | Number of inner shapes |
| `--inner-sides S` | Inner shape: number of sides for a polygon, or `c` for a circle |
| `--outer-sides S` | Container shape: number of sides for a polygon, or `c` for a circle |

### Examples

Polygons in polygons, circles in circles, and mixed:

```bash
# 3 triangles in a triangle
./polygon-packer --inner-count=3 --inner-sides=3 --outer-sides=3

# 5 circles in a circle
./polygon-packer --inner-count=5 --inner-sides=c --outer-sides=c

# 4 circles in a hexagon
./polygon-packer --inner-count=4 --inner-sides=c --outer-sides=6

# 3 squares in a circle
./polygon-packer --inner-count=3 --inner-sides=4 --outer-sides=c
```

### Output

- **Polygon container**: reports the container's **side length** in units of the inner shape's side length
- **Circle container**: reports the container's **radius** in units of the inner shape's side length (or diameter for inner circles)

### Optional flags

| Flag | Default | Description |
|------|---------|-------------|
| `--attempts N` | 1000 | Number of attempts to run. Increase to explore more packings. |
| `--tolerance F` | 1e-8 | Penalty function tolerance. Lower values reduce overlap margin but limit exploration. |
| `--finalstep F` | 0.0001 | Smallest shrink step for container size near the theoretical minimum. |
| `--cpuprofile` | off | Write a `cpu.prof` file next to the output image. |

### With Docker

```bash
docker pull ghcr.io/emerconn/polygon-packer:main
docker run --rm \
  -v "$(pwd):/work" \
  -w /work \
  ghcr.io/emerconn/polygon-packer:main \
  --inner-count=6 --inner-sides=3 --outer-sides=4
```

### CPU Profiling

```bash
go build .
./polygon-packer --inner-count=5 --inner-sides=6 --outer-sides=8 --cpuprofile
go tool pprof -http=:8080 polygon-packer cpu.prof
```

### Benchmark Testing

```bash
go build .
go test -bench=BenchmarkEvaluatorValue -benchmem -count=3
```
