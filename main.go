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
	innerType        string
	outerType        string
	innerCount       int
	innerSides       int
	containerSides   int
	attempts         int
	penaltyTolerance float64
	finalStepSize    float64
	cpuProfile       bool
	paramsPerShape   int

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

	innerDesc := fmt.Sprintf("%d_c", cfg.innerCount)
	if cfg.innerIsPolygon() {
		innerDesc = fmt.Sprintf("%d_%d", cfg.innerCount, cfg.innerSides)
	}
	outerDesc := "c"
	if cfg.outerIsPolygon() {
		outerDesc = strconv.Itoa(cfg.containerSides)
	}
	baseName := filepath.Join(outputDir, fmt.Sprintf("%s_in_%s", innerDesc, outerDesc))
	for _, outputScale := range outputScales {
		filename := uniqueFilename(fmt.Sprintf("%s_res%d.png", baseName, outputScale))
		if err := savePlot(filename, cfg, bestSide, bestValues, sizeLabel, sizeValue, outputScale); err != nil {
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

	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "-h" || arg == "--help":
			return nil, errHelp
		case arg == "--cpuprofile":
			cfg.cpuProfile = true
		case arg == "--inner-count" || strings.HasPrefix(arg, "--inner-count="):
			v, err := parseIntOption(args, &i, "--inner-count")
			if err != nil {
				return nil, err
			}
			cfg.innerCount = v
		case arg == "--inner-sides" || strings.HasPrefix(arg, "--inner-sides="):
			v, err := optionValue(args, &i, "--inner-sides")
			if err != nil {
				return nil, err
			}
			if v == "c" {
				cfg.innerType = "circle"
			} else {
				cfg.innerType = "polygon"
				cfg.innerSides, err = strconv.Atoi(v)
				if err != nil {
					return nil, fmt.Errorf("invalid --inner-sides value %q: must be a number or \"c\"", v)
				}
			}
		case arg == "--outer-sides" || strings.HasPrefix(arg, "--outer-sides="):
			v, err := optionValue(args, &i, "--outer-sides")
			if err != nil {
				return nil, err
			}
			if v == "c" {
				cfg.outerType = "circle"
			} else {
				cfg.outerType = "polygon"
				cfg.containerSides, err = strconv.Atoi(v)
				if err != nil {
					return nil, fmt.Errorf("invalid --outer-sides value %q: must be a number or \"c\"", v)
				}
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
		case arg == "--finalstep" || strings.HasPrefix(arg, "--finalstep="):
			value, err := parseFloatOption(args, &i, "--finalstep")
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
	if cfg.innerIsPolygon() && cfg.innerSides < 3 {
		return nil, fmt.Errorf("--inner-sides must be at least 3")
	}
	if cfg.outerIsPolygon() && cfg.containerSides < 3 {
		return nil, fmt.Errorf("--outer-sides must be at least 3")
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
  --inner-sides S   Inner shape: number of sides for polygon, or "c" for circle
  --outer-sides S   Container shape: number of sides for polygon, or "c" for circle

Options:
  --attempts N      Number of attempts to run (default 1000)
  --tolerance F     Overlap penalty tolerance (default 1e-8)
  --finalstep F     Smallest theoretical container-size shrink step (default 0.0001)
  --cpuprofile      Write a cpu.prof profile next to the output image

Examples:
  shape-packer --inner-count=3 --inner-sides=3 --outer-sides=3
  shape-packer --inner-count=5 --inner-sides=c --outer-sides=c
  shape-packer --inner-count=4 --inner-sides=c --outer-sides=6
  shape-packer --inner-count=3 --inner-sides=4 --outer-sides=c
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
