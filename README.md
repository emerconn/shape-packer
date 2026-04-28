# Flamethrower's polygon packer

This program can quickly solve the 2D bin packing problem for any number of any polygons inside any other polygon! It was the tool used to find all the optimal packings under the name "Ignacio Vallejo" on [Erich's Packing Center](https://erich-friedman.github.io/packing/).

## How to use

Run the Go version like this:

`go run . [n] [nsi] [nsc]`

Or build a binary first:

`go build -o polygon_packer .`

`./polygon_packer [n] [nsi] [nsc]`

The original Python script can still be run like this:

`python3 polygon_packer.py [n] [nsi] [nsc]`

- Replace `[n]` with the number of inner polygons you want to solve for
- Replace `[nsi]` with the number of sides of the inner polygons (e.g. 4 for a square)
- Replace `[nsc]` with the number of sides of the container polygon

Optional parameters:

- `--attempts`: the total number of attempts to run. Increase to explore more possible packings. Defaults to 1000.
- `--tolerance`: the tolerance for the penalty function. More penalty reduces the margin of overlap but limits exporation. Defaults to an empirical sweetspot of 0.00000001.
- `--finalstep`: the container size is decreased by a smaller factor each time, to save compute at the beginning and achieve greater precision near the end. This sets the step size of the shrinkage which would correspond to the theoretical minimum container size (which, for most packings, will not actually be reached, so keep that in mind when setting this parameter). Defaults to 0.0001.
- `--cloudprofiler`: sends runtime profiles to Google Cloud Profiler. This is also enabled automatically on Cloud Run jobs and services when `CLOUD_RUN_JOB` or `K_SERVICE` is present.

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
go tool pprof -http=:8080 ./polygon_packer cpu.prof
```

### Google Cloud Profiler

Cloud Profiler is enabled automatically in Cloud Run jobs because Cloud Run sets `CLOUD_RUN_JOB` in the container. You can also enable it explicitly:

```bash
./polygon-packer 5 6 8 --cloudprofiler
```

Useful environment variables:

- `CLOUD_PROFILER_ENABLED=false`: disable Cloud Profiler even on Cloud Run.
- `CLOUD_PROFILER_SERVICE=polygon-packer`: override the profiler service name. Defaults to `polygon-packer`, so separate Cloud Run jobs using this image are grouped under one profiler service.
- `CLOUD_PROFILER_VERSION=...`: optionally tag profiles with a deployment version.
- `CLOUD_PROFILER_PROJECT_ID=...`: required only when running outside Google Cloud.
- `CLOUD_PROFILER_MUTEX=true`: enable mutex contention profiling.
- `CLOUD_PROFILER_DEBUG=true`: enable profiler agent debug logging.

Before deploying, enable the Profiler API and make sure the Cloud Run job's service account has `roles/cloudprofiler.agent` if it is not already covered by your default service account grants:

```bash
gcloud services enable cloudprofiler.googleapis.com
gcloud projects add-iam-policy-binding "$PROJECT_ID" \
  --member "serviceAccount:$SERVICE_ACCOUNT_EMAIL" \
  --role roles/cloudprofiler.agent
```

`--cpuprofile` writes a local `cpu.prof` file and uses Go's process-wide CPU profiler, so Cloud Profiler is skipped when `--cpuprofile` is enabled.

### Benchmark Testing

```bash
go build .
go test -bench=BenchmarkEvaluatorValue -benchmem -count=3
```
