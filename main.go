package main

import (
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
	innerPolygons    int
	innerSides       int
	containerSides   int
	attempts         int
	penaltyTolerance float64
	finalStepSize    float64
	cpuProfile       bool

	unitPolygonVertices   []point
	unitPolygonVectors    []point
	unitPolygonAxisMin    float64
	unitPolygonAxisMax    float64
	unitContainerVertices []point
	unitContainerVectors  []point
	unitContainerApothem  float64
}

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
	if outputDir == "" {
		outputDir = "."
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

	sideLength := bestSide * math.Sin(math.Pi/float64(cfg.containerSides)) / math.Sin(math.Pi/float64(cfg.innerSides))
	fmt.Println("Final side length:", sideLength)

	baseName := filepath.Join(outputDir, fmt.Sprintf("%d_%d_in_%d", cfg.innerPolygons, cfg.innerSides, cfg.containerSides))
	for _, outputScale := range outputScales {
		filename := uniqueFilename(fmt.Sprintf("%s_res%d.png", baseName, outputScale))
		if err := savePlot(filename, cfg, bestSide, bestValues, sideLength, outputScale); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}
}

func parseArgs(args []string) (*config, error) {
	cfg := &config{
		attempts:         defaultAttempts,
		penaltyTolerance: defaultPenaltyTolerance,
		finalStepSize:    defaultFinalStepSize,
	}
	positional := make([]string, 0, 3)

	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "-h" || arg == "--help":
			return nil, errHelp
		case arg == "--cpuprofile":
			cfg.cpuProfile = true
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
		case arg == "--finalstep" || strings.HasPrefix(arg, "--finalstep="):
			value, err := parseFloatOption(args, &i, "--finalstep")
			if err != nil {
				return nil, err
			}
			cfg.finalStepSize = value
		case strings.HasPrefix(arg, "-"):
			return nil, fmt.Errorf("unknown option %q", arg)
		default:
			positional = append(positional, arg)
		}
	}

	if len(positional) != 3 {
		return nil, fmt.Errorf("expected 3 positional arguments, got %d", len(positional))
	}

	var err error
	cfg.innerPolygons, err = parseIntArgument(positional[0], "inner polygon count")
	if err != nil {
		return nil, err
	}
	cfg.innerSides, err = parseIntArgument(positional[1], "inner side count")
	if err != nil {
		return nil, err
	}
	cfg.containerSides, err = parseIntArgument(positional[2], "container side count")
	if err != nil {
		return nil, err
	}

	if cfg.innerPolygons <= 0 {
		return nil, fmt.Errorf("inner polygon count must be positive")
	}
	if cfg.innerSides < 3 {
		return nil, fmt.Errorf("inner polygon side count must be at least 3")
	}
	if cfg.containerSides < 3 {
		return nil, fmt.Errorf("container polygon side count must be at least 3")
	}
	if cfg.attempts <= 0 {
		return nil, fmt.Errorf("--attempts must be positive")
	}
	if cfg.penaltyTolerance < 0 {
		return nil, fmt.Errorf("--tolerance must be non-negative")
	}
	if cfg.finalStepSize <= 0 || cfg.finalStepSize >= 1 {
		return nil, fmt.Errorf("--finalstep must be between 0 and 1")
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

func parseIntArgument(text, name string) (int, error) {
	value, err := strconv.Atoi(text)
	if err != nil {
		return 0, fmt.Errorf("invalid %s %q: %w", name, text, err)
	}
	return value, nil
}

func usage() string {
	return `Usage:
  polygon-packer inner_polygons inner_sides container_sides [--attempts N] [--tolerance F] [--finalstep F] [--cpuprofile]

Arguments:
  inner_polygons   Number of inner polygons
  inner_sides      Number of sides of the inner polygons
  container_sides  Number of sides of the container polygon

Options:
  --attempts N     Number of attempts to run (default 1000)
  --tolerance F    Overlap penalty tolerance (default 1e-8)
  --finalstep F    Smallest theoretical container-size shrink step (default 0.0001)
  --cpuprofile     Write a cpu.prof profile next to the output image
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
	cfg.unitPolygonVertices = make([]point, cfg.innerSides)
	cfg.unitPolygonVectors = make([]point, cfg.innerSides)
	for i := range cfg.innerSides {
		angle := 2 * math.Pi * float64(i) / float64(cfg.innerSides)
		cfg.unitPolygonVertices[i] = point{x: math.Cos(angle), y: math.Sin(angle)}
		vectorAngle := angle + math.Pi/float64(cfg.innerSides)
		cfg.unitPolygonVectors[i] = point{x: math.Cos(vectorAngle), y: math.Sin(vectorAngle)}
	}
	cfg.unitPolygonAxisMin, cfg.unitPolygonAxisMax = projectPolygon(
		cfg.unitPolygonVertices,
		cfg.unitPolygonVectors[0].x,
		cfg.unitPolygonVectors[0].y,
	)

	cfg.unitContainerVertices = make([]point, cfg.containerSides)
	cfg.unitContainerVectors = make([]point, cfg.containerSides)
	for i := range cfg.containerSides {
		angle := 2 * math.Pi * float64(i) / float64(cfg.containerSides)
		cfg.unitContainerVertices[i] = point{x: math.Cos(angle), y: math.Sin(angle)}
		vectorAngle := angle + math.Pi/float64(cfg.containerSides)
		cfg.unitContainerVectors[i] = point{x: math.Cos(vectorAngle), y: math.Sin(vectorAngle)}
	}
	cfg.unitContainerApothem = math.Cos(math.Pi / float64(cfg.containerSides))
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
	sqrtN := math.Sqrt(float64(cfg.innerPolygons))
	dynamicSide := sqrtN * (2 + rng.Float64()*2)
	initialSide := dynamicSide
	lowestSide := sqrtN * float64(cfg.innerSides) / float64(cfg.containerSides)
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
	values := make([]float64, cfg.innerPolygons*3)
	if rng.Float64() < 0.5 {
		low := -dynamicSide / 2
		high := dynamicSide / 2
		width := high - low
		for i := range values {
			values[i] = low + rng.Float64()*width
		}
		return values
	}

	gridCount := int(math.Ceil(math.Sqrt(float64(cfg.innerPolygons))))
	grid := linspace(-dynamicSide/2*0.9, dynamicSide/2*0.9, gridCount)
	index := 0
	for y := 0; y < gridCount && index < cfg.innerPolygons; y++ {
		for x := 0; x < gridCount && index < cfg.innerPolygons; x++ {
			values[index*3] = grid[x]
			values[index*3+1] = grid[y]
			index++
		}
	}
	for i := range cfg.innerPolygons {
		values[i*3+2] = rng.Float64() * 2 * math.Pi
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
