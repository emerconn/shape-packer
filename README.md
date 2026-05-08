# emerconn's polygon packer

This program can quickly solve the 2D bin packing problem for any number of any polygons inside any other polygon! It was the tool used to find all the optimal packings under the name "Emerson Connelly" on [Erich's Packing Center](https://erich-friedman.github.io/packing/).

This is a fork of [Flamethrower's polygon-packer](https://github.com/Flamethr0wer/polygon-packer). All credit goes to him for the crazy math he did.

My fork is an optimized version in Go, roughly 40x to 45x faster than the original Python version. Both tests ran on the same PC (AMD Ryzen 9 5950X) under the same conditions.

Optimized Go version:

```bash
❯ time ./polygon-packer 3 3 3
Attempt 0
...
Attempt 999
Final side length: 1.999935429205552
./polygon-packer 3 3 3  8.55s user 1.90s system 950% cpu 1.099 total
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

Run the Go version like this:

`go run . [n] [nsi] [nsc]`

Or build a binary first:

`go build -o polygon-packer .`

`./polygon-packer [n] [nsi] [nsc]`

The original Python script can still be run like this:

`python3 polygon_packer.py [n] [nsi] [nsc]`

- Replace `[n]` with the number of inner polygons you want to solve for
- Replace `[nsi]` with the number of sides of the inner polygons (e.g. 4 for a square)
- Replace `[nsc]` with the number of sides of the container polygon

Optional parameters:

- `--attempts`: the total number of attempts to run. Increase to explore more possible packings. Defaults to 1000.
- `--tolerance`: the tolerance for the penalty function. More penalty reduces the margin of overlap but limits exporation. Defaults to an empirical sweetspot of 0.00000001.
- `--finalstep`: the container size is decreased by a smaller factor each time, to save compute at the beginning and achieve greater precision near the end. This sets the step size of the shrinkage which would correspond to the theoretical minimum container size (which, for most packings, will not actually be reached, so keep that in mind when setting this parameter). Defaults to 0.0001.
- `--cpuprofile`: writes a local `cpu.prof` file next to the output image.

### With Docker

```bash
docker pull ghcr.io/emerconn/polygon-packer:main
docker run --rm \
  -v "$(pwd):/work" \
  -w /work \
  ghcr.io/emerconn/polygon-packer:main 6 3 4
```

### CPU Profiling

```bash
go build .
./polygon-packer 5 6 8 --cpuprofile
go tool pprof -http=:8080 polygon-packer cpu.prof
```

### Benchmark Testing

```bash
go build .
go test -bench=BenchmarkEvaluatorValue -benchmem -count=3
```
