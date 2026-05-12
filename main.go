package main

import (
	"context"
	"errors"
	"fmt"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"runtime"
	"runtime/pprof"
	"slices"
	"strconv"
	"strings"
	"sync"
)

const (
	defaultAttempts         = 1000
	defaultPenaltyTolerance = 1e-8
	defaultFinalStepSize    = 0.0001
)

var (
	errHelp = errors.New("help requested")
)

type config struct {
	innerType        string
	outerType        string
	innerCount       int
	innerSides       int
	containerSides   int
	attempts         int
	penaltyTolerance float64
	finalStepSize    float64
	cpuProfile     bool
	paramsPerShape int

	unitPolygonVertices   []point
	unitPolygonVectors    []point
	unitPolygonAxisMin    float64
	unitPolygonAxisMax    float64
	unitContainerVertices []point
	unitContainerVectors  []point
	unitContainerApothem  float64
}

func (c *config) innerIsPolygon() bool { return c.innerType == "polygon" }
func (c *config) outerIsPolygon() bool { return c.outerType == "polygon" }
func (c *config) outerIsCircle() bool  { return c.outerType == "circle" }

type attemptResult struct {
	seed   int
	side   float64
	values []float64
}

func main() {
	cfg, err := parseArgs(os.Args[1:])
	if err != nil {
		if errors.Is(err, errHelp) {
			fmt.Print(usage())
			return
		}
		fmt.Fprintln(os.Stderr, err)
		fmt.Fprint(os.Stderr, usage())
		os.Exit(2)
	}

	outputDir := os.Getenv("OUTPUT_DIR")
	gcpBucket := os.Getenv("GCP_BUCKET")
	if outputDir == "" && gcpBucket == "" {
		outputDir = "."
	}
	if outputDir != "" && gcpBucket != "" {
		fmt.Fprintln(os.Stderr, "Error: OUTPUT_DIR and GCP_BUCKET cannot both be set")
		os.Exit(2)
	}

	firestoreProject := os.Getenv("FIRESTORE_PROJECT")
	firestoreDatabase := os.Getenv("FIRESTORE_DATABASE")
	if (firestoreProject != "") != (firestoreDatabase != "") {
		fmt.Fprintln(os.Stderr, "Error: FIRESTORE_PROJECT and FIRESTORE_DATABASE must both be set or both be unset")
		os.Exit(2)
	}

	var stopCPUProfile func() error
	if cfg.cpuProfile {
		profilePath := filepath.Join(outputDir, "cpu.prof")
		stopCPUProfile, err = startCPUProfile(profilePath)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		defer func() {
			if stopCPUProfile == nil {
				return
			}
			if err := stopCPUProfile(); err != nil {
				fmt.Fprintln(os.Stderr, err)
			}
		}()
		fmt.Println("CPU profile enabled:", profilePath)
	}

	results := runAttempts(cfg)
	if stopCPUProfile != nil {
		if err := stopCPUProfile(); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		stopCPUProfile = nil
	}

	bestSide := math.Inf(1)
	var bestValues []float64
	for _, result := range results {
		if result.side < bestSide {
			bestSide = result.side
			bestValues = result.values
		}
	}

	var sizeLabel string
	var sizeValue float64
	if cfg.outerIsPolygon() {
		sizeLabel = "side length"
		containerSideLength := bestSide * 2 * math.Sin(math.Pi/float64(cfg.containerSides))
		if cfg.innerIsPolygon() {
			sizeValue = containerSideLength / (2 * math.Sin(math.Pi/float64(cfg.innerSides)))
		} else {
			sizeValue = containerSideLength
		}
	} else {
		sizeLabel = "radius"
		if cfg.innerIsPolygon() {
			sizeValue = bestSide / (2 * math.Sin(math.Pi/float64(cfg.innerSides)))
		} else {
			sizeValue = bestSide
		}
	}
	fmt.Println("Final", sizeLabel+":", sizeValue)

	innerDesc := fmt.Sprintf("%d_%d", cfg.innerCount, cfg.innerSides)
	outerDesc := strconv.Itoa(cfg.containerSides)
	objectPrefix := fmt.Sprintf("%s_in_%s", innerDesc, outerDesc)

	imageURLs := map[string]string{}
	if gcpBucket != "" {
		ctx := context.Background()
		gcsClient, err := newGCSClient(ctx)
		if err != nil {
			fmt.Fprintln(os.Stderr, "Error: connecting to Cloud Storage:", err)
			os.Exit(1)
		}
		defer func() {
			if err := gcsClient.Close(); err != nil {
				fmt.Fprintln(os.Stderr, "Error: closing Cloud Storage client:", err)
			}
		}()
		for _, outputScale := range outputScales {
			objectName := fmt.Sprintf("%s_res%d.png", objectPrefix, outputScale)
			url, err := uploadPlot(ctx, gcsClient, gcpBucket, objectName, cfg, bestSide, bestValues, sizeLabel, sizeValue, outputScale)
			if err != nil {
				fmt.Fprintln(os.Stderr, "Error: uploading plot:", err)
				os.Exit(1)
			}
			imageURLs[fmt.Sprintf("res%d", outputScale)] = url
		}
	} else {
		baseName := filepath.Join(outputDir, objectPrefix)
		for _, outputScale := range outputScales {
			filename := uniqueFilename(fmt.Sprintf("%s_res%d.png", baseName, outputScale))
			if err := writePlotToFile(filename, cfg, bestSide, bestValues, sizeLabel, sizeValue, outputScale); err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(1)
			}
		}
	}

	if firestoreProject != "" {
		ctx := context.Background()
		fsClient, err := newFirestoreClient(ctx, firestoreProject, firestoreDatabase)
		if err != nil {
			fmt.Fprintln(os.Stderr, "Error: connecting to Firestore:", err)
			os.Exit(1)
		}
		defer func() {
			if err := fsClient.Close(); err != nil {
				fmt.Fprintln(os.Stderr, "Error: closing Firestore client:", err)
			}
		}()
		if err := saveRun(ctx, fsClient, cfg, sizeLabel, sizeValue, imageURLs); err != nil {
			fmt.Fprintln(os.Stderr, "Error: saving to Firestore:", err)
			os.Exit(1)
		}
		fmt.Println("Saved to Firestore:", packingDocID(cfg))
	}
}

func parseArgs(args []string) (*config, error) {
	cfg := &config{
		attempts:         defaultAttempts,
		penaltyTolerance: defaultPenaltyTolerance,
		finalStepSize:    defaultFinalStepSize,
	}

	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "-h" || arg == "--help":
			return nil, errHelp
		case arg == "--cpu-profile":
			cfg.cpuProfile = true
		case arg == "--inner-count" || strings.HasPrefix(arg, "--inner-count="):
			v, err := parseIntOption(args, &i, "--inner-count")
			if err != nil {
				return nil, err
			}
			cfg.innerCount = v
		case arg == "--inner-sides" || strings.HasPrefix(arg, "--inner-sides="):
			v, err := parseIntOption(args, &i, "--inner-sides")
			if err != nil {
				return nil, err
			}
			cfg.innerSides = v
			if v == 0 {
				cfg.innerType = "circle"
			} else if v < 3 {
				return nil, fmt.Errorf("--inner-sides must be 0 (circle) or at least 3")
			} else {
				cfg.innerType = "polygon"
			}
		case arg == "--outer-sides" || strings.HasPrefix(arg, "--outer-sides="):
			v, err := parseIntOption(args, &i, "--outer-sides")
			if err != nil {
				return nil, err
			}
			cfg.containerSides = v
			if v == 0 {
				cfg.outerType = "circle"
			} else if v < 3 {
				return nil, fmt.Errorf("--outer-sides must be 0 (circle) or at least 3")
			} else {
				cfg.outerType = "polygon"
			}
		case arg == "--attempts" || strings.HasPrefix(arg, "--attempts="):
			value, err := parseIntOption(args, &i, "--attempts")
			if err != nil {
				return nil, err
			}
			cfg.attempts = value
		case arg == "--tolerance" || strings.HasPrefix(arg, "--tolerance="):
			value, err := parseFloatOption(args, &i, "--tolerance")
			if err != nil {
				return nil, err
			}
			cfg.penaltyTolerance = value
		case arg == "--final-step" || strings.HasPrefix(arg, "--final-step="):
			value, err := parseFloatOption(args, &i, "--final-step")
			if err != nil {
				return nil, err
			}
			cfg.finalStepSize = value
		case strings.HasPrefix(arg, "-"):
			return nil, fmt.Errorf("unknown option %q", arg)
		default:
			return nil, fmt.Errorf("unexpected argument %q", arg)
		}
	}

	if cfg.innerType == "" {
		return nil, fmt.Errorf("--inner-sides is required")
	}
	if cfg.outerType == "" {
		return nil, fmt.Errorf("--outer-sides is required")
	}
	if cfg.innerCount <= 0 {
		return nil, fmt.Errorf("--inner-count is required and must be positive")
	}
	if cfg.attempts <= 0 {
		return nil, fmt.Errorf("--attempts must be positive")
	}
	if cfg.penaltyTolerance < 0 {
		return nil, fmt.Errorf("--tolerance must be non-negative")
	}
	if cfg.finalStepSize <= 0 || cfg.finalStepSize >= 1 {
		return nil, fmt.Errorf("--final-step must be between 0 and 1")
	}

	if cfg.innerIsPolygon() {
		cfg.paramsPerShape = 3
	} else {
		cfg.paramsPerShape = 2
	}

	cfg.precompute()
	return cfg, nil
}

func parseIntOption(args []string, i *int, name string) (int, error) {
	text, err := optionValue(args, i, name)
	if err != nil {
		return 0, err
	}
	value, err := strconv.Atoi(text)
	if err != nil {
		return 0, fmt.Errorf("invalid %s value %q: %w", name, text, err)
	}
	return value, nil
}

func parseFloatOption(args []string, i *int, name string) (float64, error) {
	text, err := optionValue(args, i, name)
	if err != nil {
		return 0, err
	}
	value, err := strconv.ParseFloat(text, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid %s value %q: %w", name, text, err)
	}
	return value, nil
}

func optionValue(args []string, i *int, name string) (string, error) {
	if value, ok := strings.CutPrefix(args[*i], name+"="); ok {
		return value, nil
	}
	*i++
	if *i >= len(args) {
		return "", fmt.Errorf("%s requires a value", name)
	}
	return args[*i], nil
}

func usage() string {
	return `Usage:
  shape-packer --inner-count=N --inner-sides=S --outer-sides=S [options]

Required:
  --inner-count N   Number of inner shapes
  --inner-sides S   Inner shape: number of sides (0 for circle, 3+ for polygon)
  --outer-sides S   Container shape: number of sides (0 for circle, 3+ for polygon)

Options:
  --attempts N      Number of attempts to run (default 1000)
  --tolerance F     Overlap penalty tolerance (default 1e-8)
  --final-step F    Smallest theoretical container-size shrink step (default 0.0001)
  --cpu-profile     Write a cpu.prof profile next to the output image

Environment:
  OUTPUT_DIR          Directory for output PNG files (default: current directory)
  GCP_BUCKET          GCS bucket name to upload PNG files (mutually exclusive with OUTPUT_DIR)
  FIRESTORE_PROJECT   GCP project ID for Firestore (requires FIRESTORE_DATABASE)
  FIRESTORE_DATABASE  Firestore database ID (requires FIRESTORE_PROJECT)

Examples:
  shape-packer --inner-count=3 --inner-sides=3 --outer-sides=3
  shape-packer --inner-count=5 --inner-sides=0 --outer-sides=0
  shape-packer --inner-count=4 --inner-sides=0 --outer-sides=6
  shape-packer --inner-count=3 --inner-sides=4 --outer-sides=0
`
}

func uniqueFilename(base string) string {
	if _, err := os.Stat(base); os.IsNotExist(err) {
		return base
	}
	ext := filepath.Ext(base)
	name := strings.TrimSuffix(base, ext)
	for i := 1; ; i++ {
		candidate := fmt.Sprintf("%s_%d%s", name, i, ext)
		if _, err := os.Stat(candidate); os.IsNotExist(err) {
			return candidate
		}
	}
}

func startCPUProfile(filename string) (func() error, error) {
	if err := os.MkdirAll(filepath.Dir(filename), 0755); err != nil {
		return nil, fmt.Errorf("create profile directory %s: %w", filepath.Dir(filename), err)
	}

	file, err := os.Create(filename)
	if err != nil {
		return nil, fmt.Errorf("create CPU profile %s: %w", filename, err)
	}
	if err := pprof.StartCPUProfile(file); err != nil {
		closeErr := file.Close()
		if closeErr != nil {
			return nil, fmt.Errorf("start CPU profile %s: %w; close profile: %v", filename, err, closeErr)
		}
		return nil, fmt.Errorf("start CPU profile %s: %w", filename, err)
	}

	stopped := false
	return func() error {
		if !stopped {
			pprof.StopCPUProfile()
			stopped = true
		}
		if err := file.Close(); err != nil {
			return fmt.Errorf("close CPU profile %s: %w", filename, err)
		}
		return nil
	}, nil
}

func (cfg *config) precompute() {
	if cfg.innerIsPolygon() {
		cfg.unitPolygonVertices = make([]point, cfg.innerSides)
		cfg.unitPolygonVectors = make([]point, cfg.innerSides)
		for i := range cfg.innerSides {
			angle := 2 * math.Pi * float64(i) / float64(cfg.innerSides)
			cfg.unitPolygonVertices[i] = point{x: math.Cos(angle), y: math.Sin(angle)}
			vectorAngle := angle + math.Pi / float64(cfg.innerSides)
			cfg.unitPolygonVectors[i] = point{x: math.Cos(vectorAngle), y: math.Sin(vectorAngle)}
		}
		cfg.unitPolygonAxisMin, cfg.unitPolygonAxisMax = projectPolygon(
			cfg.unitPolygonVertices,
			cfg.unitPolygonVectors[0].x,
			cfg.unitPolygonVectors[0].y,
		)
	}

	if cfg.outerIsPolygon() {
		cfg.unitContainerVertices = make([]point, cfg.containerSides)
		cfg.unitContainerVectors = make([]point, cfg.containerSides)
		for i := range cfg.containerSides {
			angle := 2 * math.Pi * float64(i) / float64(cfg.containerSides)
			cfg.unitContainerVertices[i] = point{x: math.Cos(angle), y: math.Sin(angle)}
			vectorAngle := angle + math.Pi / float64(cfg.containerSides)
			cfg.unitContainerVectors[i] = point{x: math.Cos(vectorAngle), y: math.Sin(vectorAngle)}
		}
		cfg.unitContainerApothem = math.Cos(math.Pi / float64(cfg.containerSides))
	}
}

func runAttempts(cfg *config) []attemptResult {
	workers := runtime.NumCPU()
	if workers > cfg.attempts {
		workers = cfg.attempts
	}

	jobs := make(chan int)
	results := make([]attemptResult, cfg.attempts)
	var wg sync.WaitGroup
	wg.Add(workers)
	for range workers {
		go func() {
			defer wg.Done()
			for seed := range jobs {
				results[seed] = repetition(seed, cfg)
			}
		}()
	}

	for seed := range cfg.attempts {
		jobs <- seed
	}
	close(jobs)
	wg.Wait()
	return results
}

func repetition(seed int, cfg *config) attemptResult {
	fmt.Println("Attempt", seed)

	rng := rand.New(rand.NewSource(int64(seed)))
	sqrtN := math.Sqrt(float64(cfg.innerCount))
	dynamicSide := sqrtN * (2 + rng.Float64()*2)
	initialSide := dynamicSide

	var lowestSide float64
	if cfg.innerIsPolygon() && cfg.outerIsPolygon() {
		lowestSide = sqrtN * float64(cfg.innerSides) / float64(cfg.containerSides)
	} else {
		lowestSide = sqrtN
	}
	sideRange := initialSide - lowestSide

	x0 := initialValues(rng, cfg, dynamicSide)
	lastValidX := slices.Clone(x0)
	lastValidSide := dynamicSide

	for {
		objective := newPackingObjective(cfg, dynamicSide)
		minimized := minimizeLBFGSWithGradient(x0, objective.value, objective.gradient, 1e-8)

		multiplier := 1 - cfg.finalStepSize - (dynamicSide-lowestSide)*(0.01-cfg.finalStepSize)/sideRange
		if minimized.fun < cfg.penaltyTolerance {
			lastValidX = slices.Clone(minimized.x)
			lastValidSide = dynamicSide
			x0 = scaleFloat64s(minimized.x, multiplier)
			dynamicSide *= multiplier
			continue
		}

		basinResult := basinHopping(minimized, objective.value, objective.gradient, rng)
		if basinResult.fun < cfg.penaltyTolerance {
			lastValidX = slices.Clone(basinResult.x)
			lastValidSide = dynamicSide
			x0 = scaleFloat64s(basinResult.x, multiplier)
			dynamicSide *= multiplier
			continue
		}
		break
	}

	return attemptResult{seed: seed, side: lastValidSide, values: lastValidX}
}

func initialValues(rng *rand.Rand, cfg *config, dynamicSide float64) []float64 {
	values := make([]float64, cfg.innerCount*cfg.paramsPerShape)
	if rng.Float64() < 0.5 {
		low := -dynamicSide / 2
		high := dynamicSide / 2
		width := high - low
		for i := range values {
			values[i] = low + rng.Float64()*width
		}
		return values
	}

	gridCount := int(math.Ceil(math.Sqrt(float64(cfg.innerCount))))
	grid := linspace(-dynamicSide/2*0.9, dynamicSide/2*0.9, gridCount)
	index := 0
	for y := 0; y < gridCount && index < cfg.innerCount; y++ {
		for x := 0; x < gridCount && index < cfg.innerCount; x++ {
			values[index*cfg.paramsPerShape] = grid[x]
			values[index*cfg.paramsPerShape+1] = grid[y]
			index++
		}
	}
	if cfg.innerIsPolygon() {
		for i := range cfg.innerCount {
			values[i*cfg.paramsPerShape+2] = rng.Float64() * 2 * math.Pi
		}
	}
	return values
}

func linspace(start, stop float64, count int) []float64 {
	values := make([]float64, count)
	if count == 1 {
		values[0] = start
		return values
	}
	step := (stop - start) / float64(count-1)
	for i := range count {
		values[i] = start + float64(i)*step
	}
	return values
}
