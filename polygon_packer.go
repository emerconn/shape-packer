package main

import (
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"runtime"
	"runtime/pprof"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"

	"cloud.google.com/go/profiler"
)

const (
	defaultAttempts         = 1000
	defaultPenaltyTolerance = 1e-8
	defaultFinalStepSize    = 0.0001

	lbfgsHistorySize       = 10
	lbfgsMaxIterations     = 1000
	lbfgsMaxFunctionEvals  = 15000
	lbfgsGradientEps       = 1e-8
	lbfgsGradientTolerance = 1e-5

	basinHoppingIterations = 50
	basinHoppingTemp       = 0.1
	basinHoppingStepSize   = 0.1

	spatialGridThreshold = 96
	spatialGridCellSize  = 2.0
)

var (
	version      = "dev"
	errHelp      = errors.New("help requested")
	outputScales = [...]int{1, 2, 4, 6}
)

type point struct {
	x, y float64
}

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

type evaluator struct {
	cfg        *config
	polys      []point
	vectors    []point
	cells      []gridCell
	nextInCell []int
	cellHeads  map[gridCell]int
	usedCells  []gridCell
}

type optResult struct {
	x          []float64
	fun        float64
	iterations int
	evals      int
}

type attemptResult struct {
	seed   int
	side   float64
	values []float64
}

type gridCell struct {
	x, y int
}

func main() {
	// Initialize Google Cloud Profiler
	if err := profiler.Start(profiler.Config{
		Service:        "polygon-packer",
		ServiceVersion: version,
		ProjectID:      "basic-bison-138323",
		DebugLogging:   true,
	}); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: Failed to start profiler (this is normal for local runs): %v\n", err)
	}

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
	*i = *i + 1
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
  polygon_packer inner_polygons inner_sides container_sides [--attempts N] [--tolerance F] [--finalstep F] [--cpuprofile]

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
	for i := 0; i < cfg.innerSides; i++ {
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
	for i := 0; i < cfg.containerSides; i++ {
		angle := 2 * math.Pi * float64(i) / float64(cfg.containerSides)
		cfg.unitContainerVertices[i] = point{x: math.Cos(angle), y: math.Sin(angle)}
		vectorAngle := angle + math.Pi/float64(cfg.containerSides)
		cfg.unitContainerVectors[i] = point{x: math.Cos(vectorAngle), y: math.Sin(vectorAngle)}
	}
	cfg.unitContainerApothem = math.Cos(math.Pi / float64(cfg.containerSides))
}

func newEvaluator(cfg *config) *evaluator {
	return &evaluator{
		cfg:        cfg,
		polys:      make([]point, cfg.innerPolygons*cfg.innerSides),
		vectors:    make([]point, cfg.innerPolygons*cfg.innerSides),
		cells:      make([]gridCell, cfg.innerPolygons),
		nextInCell: make([]int, cfg.innerPolygons),
		cellHeads:  make(map[gridCell]int, cfg.innerPolygons),
		usedCells:  make([]gridCell, 0, cfg.innerPolygons),
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
	for worker := 0; worker < workers; worker++ {
		go func() {
			defer wg.Done()
			for seed := range jobs {
				results[seed] = repetition(seed, cfg)
			}
		}()
	}

	for seed := 0; seed < cfg.attempts; seed++ {
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
	eval := newEvaluator(cfg)

	for {
		sideForObjective := dynamicSide
		minimized := minimizeLBFGS(x0, func(x []float64) float64 {
			return eval.value(x, sideForObjective)
		}, 1e-8)

		multiplier := 1 - cfg.finalStepSize - (dynamicSide-lowestSide)*(0.01-cfg.finalStepSize)/sideRange
		if minimized.fun < cfg.penaltyTolerance {
			lastValidX = slices.Clone(minimized.x)
			lastValidSide = dynamicSide
			x0 = scaleFloat64s(minimized.x, multiplier)
			dynamicSide *= multiplier
			continue
		}

		basinResult := basinHopping(minimized, func(x []float64) float64 {
			return eval.value(x, sideForObjective)
		}, rng)
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
	for i := 0; i < cfg.innerPolygons; i++ {
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
	for i := 0; i < count; i++ {
		values[i] = start + float64(i)*step
	}
	return values
}

func (e *evaluator) value(values []float64, side float64) float64 {
	cfg := e.cfg
	innerSides := cfg.innerSides
	containerLimit := cfg.unitContainerApothem * side
	penalty := 0.0
	for i := 0; i < cfg.innerPolygons; i++ {
		base := i * innerSides
		valueBase := i * 3
		polygon := e.polys[base : base+innerSides]
		vectors := e.vectors[base : base+innerSides]
		penalty += transformPolygonAndVectors(
			values[valueBase],
			values[valueBase+1],
			values[valueBase+2],
			cfg,
			containerLimit,
			polygon,
			vectors,
		)
	}

	if cfg.innerPolygons >= spatialGridThreshold {
		penalty += e.spatialCollisionPenalty(values)
	} else {
		for i := 0; i < cfg.innerPolygons; i++ {
			valueBaseI := i * 3
			centerIX := values[valueBaseI]
			centerIY := values[valueBaseI+1]
			baseI := i * innerSides
			polygonI := e.polys[baseI : baseI+innerSides]
			vectorsI := e.vectors[baseI : baseI+innerSides]
			for j := i + 1; j < cfg.innerPolygons; j++ {
				valueBaseJ := j * 3
				centerJX := values[valueBaseJ]
				centerJY := values[valueBaseJ+1]
				dx := centerIX - centerJX
				dy := centerIY - centerJY
				if dx*dx+dy*dy >= 4 {
					continue
				}

				baseJ := j * innerSides
				polygonJ := e.polys[baseJ : baseJ+innerSides]
				vectorsJ := e.vectors[baseJ : baseJ+innerSides]
				collision := true
				minOverlap := 1e20

				// For congruent regular polygons, projection width is constant across
				// another polygon's axis family; only the center projection changes.
				firstVectorI := vectorsI[0]
				firstCenterProjectionJ := centerJX*firstVectorI.x + centerJY*firstVectorI.y
				firstMinJ, firstMaxJ := projectPolygon(polygonJ, firstVectorI.x, firstVectorI.y)
				axisIMinJ := firstMinJ - firstCenterProjectionJ
				axisIMaxJ := firstMaxJ - firstCenterProjectionJ
				for axis := 0; axis < innerSides; axis++ {
					vector := vectorsI[axis]
					axisX := vector.x
					axisY := vector.y
					centerProjection := centerIX*axisX + centerIY*axisY
					minI := centerProjection + cfg.unitPolygonAxisMin
					maxI := centerProjection + cfg.unitPolygonAxisMax
					centerProjectionJ := centerJX*axisX + centerJY*axisY
					minJ := centerProjectionJ + axisIMinJ
					maxJ := centerProjectionJ + axisIMaxJ
					overlap := intervalOverlap(minI, maxI, minJ, maxJ)
					if overlap <= 0 {
						collision = false
						break
					}
					if overlap < minOverlap {
						minOverlap = overlap
					}
				}
				if !collision {
					continue
				}

				firstVectorJ := vectorsJ[0]
				firstCenterProjectionI := centerIX*firstVectorJ.x + centerIY*firstVectorJ.y
				firstMinI, firstMaxI := projectPolygon(polygonI, firstVectorJ.x, firstVectorJ.y)
				axisJMinI := firstMinI - firstCenterProjectionI
				axisJMaxI := firstMaxI - firstCenterProjectionI
				for axis := 0; axis < innerSides; axis++ {
					vector := vectorsJ[axis]
					axisX := vector.x
					axisY := vector.y
					centerProjectionI := centerIX*axisX + centerIY*axisY
					minI := centerProjectionI + axisJMinI
					maxI := centerProjectionI + axisJMaxI
					centerProjection := centerJX*axisX + centerJY*axisY
					minJ := centerProjection + cfg.unitPolygonAxisMin
					maxJ := centerProjection + cfg.unitPolygonAxisMax
					overlap := intervalOverlap(minI, maxI, minJ, maxJ)
					if overlap <= 0 {
						collision = false
						break
					}
					if overlap < minOverlap {
						minOverlap = overlap
					}
				}

				if collision {
					penalty += minOverlap * minOverlap
				}
			}
		}
	}

	return penalty
}

func (e *evaluator) spatialCollisionPenalty(values []float64) float64 {
	e.buildSpatialGrid(values)

	penalty := 0.0
	for i := 0; i < e.cfg.innerPolygons; i++ {
		cell := e.cells[i]
		for dx := -1; dx <= 1; dx++ {
			for dy := -1; dy <= 1; dy++ {
				head, ok := e.cellHeads[gridCell{x: cell.x + dx, y: cell.y + dy}]
				if !ok {
					continue
				}
				for j := head; j >= 0; j = e.nextInCell[j] {
					if j > i {
						penalty += e.pairPenalty(values, i, j)
					}
				}
			}
		}
	}
	return penalty
}

func (e *evaluator) buildSpatialGrid(values []float64) {
	for _, cell := range e.usedCells {
		delete(e.cellHeads, cell)
	}
	e.usedCells = e.usedCells[:0]

	for i := 0; i < e.cfg.innerPolygons; i++ {
		valueBase := i * 3
		cell := gridCell{
			x: int(math.Floor(values[valueBase] / spatialGridCellSize)),
			y: int(math.Floor(values[valueBase+1] / spatialGridCellSize)),
		}
		e.cells[i] = cell

		head, ok := e.cellHeads[cell]
		if !ok {
			head = -1
			e.usedCells = append(e.usedCells, cell)
		}
		e.nextInCell[i] = head
		e.cellHeads[cell] = i
	}
}

func (e *evaluator) pairPenalty(values []float64, i, j int) float64 {
	cfg := e.cfg
	innerSides := cfg.innerSides

	valueBaseI := i * 3
	centerIX := values[valueBaseI]
	centerIY := values[valueBaseI+1]
	valueBaseJ := j * 3
	centerJX := values[valueBaseJ]
	centerJY := values[valueBaseJ+1]

	dx := centerIX - centerJX
	dy := centerIY - centerJY
	if dx*dx+dy*dy >= 4 {
		return 0
	}

	baseI := i * innerSides
	polygonI := e.polys[baseI : baseI+innerSides]
	vectorsI := e.vectors[baseI : baseI+innerSides]
	baseJ := j * innerSides
	polygonJ := e.polys[baseJ : baseJ+innerSides]
	vectorsJ := e.vectors[baseJ : baseJ+innerSides]
	minOverlap := 1e20

	// For congruent regular polygons, projection width is constant across
	// another polygon's axis family; only the center projection changes.
	firstVectorI := vectorsI[0]
	firstCenterProjectionJ := centerJX*firstVectorI.x + centerJY*firstVectorI.y
	firstMinJ, firstMaxJ := projectPolygon(polygonJ, firstVectorI.x, firstVectorI.y)
	axisIMinJ := firstMinJ - firstCenterProjectionJ
	axisIMaxJ := firstMaxJ - firstCenterProjectionJ
	for axis := 0; axis < innerSides; axis++ {
		vector := vectorsI[axis]
		axisX := vector.x
		axisY := vector.y
		centerProjection := centerIX*axisX + centerIY*axisY
		minI := centerProjection + cfg.unitPolygonAxisMin
		maxI := centerProjection + cfg.unitPolygonAxisMax
		centerProjectionJ := centerJX*axisX + centerJY*axisY
		minJ := centerProjectionJ + axisIMinJ
		maxJ := centerProjectionJ + axisIMaxJ
		overlap := intervalOverlap(minI, maxI, minJ, maxJ)
		if overlap <= 0 {
			return 0
		}
		if overlap < minOverlap {
			minOverlap = overlap
		}
	}

	firstVectorJ := vectorsJ[0]
	firstCenterProjectionI := centerIX*firstVectorJ.x + centerIY*firstVectorJ.y
	firstMinI, firstMaxI := projectPolygon(polygonI, firstVectorJ.x, firstVectorJ.y)
	axisJMinI := firstMinI - firstCenterProjectionI
	axisJMaxI := firstMaxI - firstCenterProjectionI
	for axis := 0; axis < innerSides; axis++ {
		vector := vectorsJ[axis]
		axisX := vector.x
		axisY := vector.y
		centerProjectionI := centerIX*axisX + centerIY*axisY
		minI := centerProjectionI + axisJMinI
		maxI := centerProjectionI + axisJMaxI
		centerProjection := centerJX*axisX + centerJY*axisY
		minJ := centerProjection + cfg.unitPolygonAxisMin
		maxJ := centerProjection + cfg.unitPolygonAxisMax
		overlap := intervalOverlap(minI, maxI, minJ, maxJ)
		if overlap <= 0 {
			return 0
		}
		if overlap < minOverlap {
			minOverlap = overlap
		}
	}

	return minOverlap * minOverlap
}

func transformPolygonAndVectors(x, y, angle float64, cfg *config, containerLimit float64, polygonOut, vectorOut []point) float64 {
	sinAngle, cosAngle := math.Sincos(angle)
	penalty := 0.0

	for i := range cfg.unitPolygonVertices {
		vertex := cfg.unitPolygonVertices[i]
		polygonX := x + (vertex.x*cosAngle - vertex.y*sinAngle)
		polygonY := y + (vertex.x*sinAngle + vertex.y*cosAngle)
		polygonOut[i] = point{x: polygonX, y: polygonY}

		for _, vector := range cfg.unitContainerVectors {
			distance := polygonX*vector.x + polygonY*vector.y
			if distance > containerLimit {
				diff := distance - containerLimit
				penalty += diff * diff
			}
		}

		baseVector := cfg.unitPolygonVectors[i]
		vectorOut[i] = point{
			x: baseVector.x*cosAngle - baseVector.y*sinAngle,
			y: baseVector.x*sinAngle + baseVector.y*cosAngle,
		}
	}

	return penalty
}

func transformPolygon(x, y, angle float64, vertices []point, out []point) {
	sinAngle, cosAngle := math.Sincos(angle)
	for i, vertex := range vertices {
		out[i] = point{
			x: x + (vertex.x*cosAngle - vertex.y*sinAngle),
			y: y + (vertex.x*sinAngle + vertex.y*cosAngle),
		}
	}
}

func rotateVectors(angle float64, vectors []point, out []point) {
	sinAngle, cosAngle := math.Sincos(angle)
	for i, vector := range vectors {
		out[i] = point{
			x: vector.x*cosAngle - vector.y*sinAngle,
			y: vector.x*sinAngle + vector.y*cosAngle,
		}
	}
}

func projectPolygon(vertices []point, axisX, axisY float64) (float64, float64) {
	first := vertices[0]
	firstDot := first.x*axisX + first.y*axisY
	minValue := firstDot
	maxValue := firstDot
	for i := 1; i < len(vertices); i++ {
		vertex := vertices[i]
		dot := vertex.x*axisX + vertex.y*axisY
		if dot < minValue {
			minValue = dot
		}
		if dot > maxValue {
			maxValue = dot
		}
	}
	return minValue, maxValue
}

func intervalOverlap(minA, maxA, minB, maxB float64) float64 {
	upper := maxA
	if maxB < upper {
		upper = maxB
	}
	lower := minA
	if minB > lower {
		lower = minB
	}
	return upper - lower
}

func minimizeLBFGS(x0 []float64, objective func([]float64) float64, tol float64) optResult {
	n := len(x0)
	x := slices.Clone(x0)
	fun := objective(x)
	evals := 1
	if fun == 0 {
		return optResult{x: x, fun: fun, evals: evals}
	}
	gradient := make([]float64, n)
	gradientEvals := finiteDifferenceGradient(objective, x, fun, gradient, lbfgsMaxFunctionEvals-evals)
	evals += gradientEvals

	sHistory := make([][]float64, 0, lbfgsHistorySize)
	yHistory := make([][]float64, 0, lbfgsHistorySize)
	rhoHistory := make([]float64, 0, lbfgsHistorySize)
	iterations := 0

	for iterations < lbfgsMaxIterations && evals < lbfgsMaxFunctionEvals {
		if maxAbs(gradient) <= lbfgsGradientTolerance {
			break
		}

		direction := lbfgsDirection(gradient, sHistory, yHistory, rhoHistory)
		if dot(direction, gradient) >= 0 || !allFinite(direction) {
			for i := range direction {
				direction[i] = -gradient[i]
			}
		}

		newX, newFun, lineEvals, ok := lineSearch(objective, x, fun, gradient, direction, lbfgsMaxFunctionEvals-evals)
		evals += lineEvals
		if !ok {
			break
		}

		newGradient := make([]float64, n)
		gradientEvals = finiteDifferenceGradient(objective, newX, newFun, newGradient, lbfgsMaxFunctionEvals-evals)
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
		x = newX
		fun = newFun
		gradient = newGradient
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

func lbfgsDirection(gradient []float64, sHistory, yHistory [][]float64, rhoHistory []float64) []float64 {
	q := slices.Clone(gradient)
	alpha := make([]float64, len(sHistory))
	for i := len(sHistory) - 1; i >= 0; i-- {
		alpha[i] = rhoHistory[i] * dot(sHistory[i], q)
		axpy(q, yHistory[i], -alpha[i])
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

	r := make([]float64, len(q))
	for i := range q {
		r[i] = q[i] * scale
	}
	for i := 0; i < len(sHistory); i++ {
		beta := rhoHistory[i] * dot(yHistory[i], r)
		axpy(r, sHistory[i], alpha[i]-beta)
	}
	for i := range r {
		r[i] = -r[i]
	}
	return r
}

func lineSearch(objective func([]float64) float64, x []float64, f0 float64, gradient []float64, direction []float64, maxEvals int) ([]float64, float64, int, bool) {
	derivative := dot(gradient, direction)
	if derivative >= 0 || !isFinite(derivative) || maxEvals <= 0 {
		return nil, 0, 0, false
	}

	const armijo = 1e-4
	step := 1.0
	evals := 0
	trial := make([]float64, len(x))
	var bestX []float64
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
				if bestX == nil {
					bestX = make([]float64, len(x))
				}
				copy(bestX, trial)
				bestFun = trialFun
				improved = true
			}
			if trialFun <= f0+armijo*step*derivative {
				return trial, trialFun, evals, true
			}
		}
		step *= 0.5
	}

	if improved {
		return bestX, bestFun, evals, true
	}
	return nil, 0, evals, false
}

func basinHopping(current optResult, objective func([]float64) float64, rng *rand.Rand) optResult {
	best := optResult{
		x:          slices.Clone(current.x),
		fun:        current.fun,
		iterations: current.iterations,
		evals:      current.evals,
	}

	for i := 0; i < basinHoppingIterations; i++ {
		trial := slices.Clone(current.x)
		for j := range trial {
			trial[j] += (rng.Float64()*2 - 1) * basinHoppingStepSize
		}

		minimized := minimizeLBFGS(trial, objective, 1e-8)
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

func savePlot(filename string, cfg *config, side float64, values []float64, sideLength float64, outputScale int) error {
	if err := os.MkdirAll(filepath.Dir(filename), 0755); err != nil {
		return fmt.Errorf("create directory %s: %w", filepath.Dir(filename), err)
	}

	const (
		baseWidth     = 640
		baseHeight    = 480
		baseTitleArea = 40
		baseMargin    = 24
	)
	width := baseWidth * outputScale
	height := baseHeight * outputScale
	titleArea := baseTitleArea * outputScale
	margin := baseMargin * outputScale

	img := image.NewRGBA(image.Rect(0, 0, width, height))
	draw.Draw(img, img.Bounds(), image.NewUniform(color.White), image.Point{}, draw.Src)

	polygons := make([][]point, cfg.innerPolygons)
	allPoints := make([]point, 0, cfg.containerSides+cfg.innerPolygons*cfg.innerSides)
	plotCos, plotSin := plotRotation(cfg.containerSides)
	container := make([]point, cfg.containerSides)
	for i, vertex := range cfg.unitContainerVertices {
		container[i] = rotatePoint(point{x: vertex.x * side, y: vertex.y * side}, plotCos, plotSin)
		allPoints = append(allPoints, container[i])
	}
	for i := 0; i < cfg.innerPolygons; i++ {
		polygon := make([]point, cfg.innerSides)
		transformPolygon(values[i*3], values[i*3+1], values[i*3+2], cfg.unitPolygonVertices, polygon)
		rotatePoints(polygon, plotCos, plotSin)
		polygons[i] = polygon
		allPoints = append(allPoints, polygon...)
	}

	minX, maxX, minY, maxY := bounds(allPoints)
	spanX := maxX - minX
	spanY := maxY - minY
	if spanX == 0 {
		spanX = 1
	}
	if spanY == 0 {
		spanY = 1
	}
	plotHeight := height - titleArea - margin
	scaleX := float64(width-2*margin) / spanX
	scaleY := float64(plotHeight-2*margin) / spanY
	scale := min(scaleX, scaleY) * 0.95
	centerX := (minX + maxX) / 2
	centerY := (minY + maxY) / 2
	screenPoint := func(p point) image.Point {
		x := float64(width)/2 + (p.x-centerX)*scale
		y := float64(titleArea) + float64(plotHeight)/2 - (p.y-centerY)*scale
		return image.Point{X: int(math.Round(x)), Y: int(math.Round(y))}
	}

	containerScreen := toScreenPolygon(container, screenPoint)
	drawPolyline(img, containerScreen, true, color.RGBA{A: 255})
	for _, polygon := range polygons {
		screenPolygon := toScreenPolygon(polygon, screenPoint)
		fillPolygon(img, screenPolygon, color.RGBA{R: 204, G: 204, B: 204, A: 255})
		drawPolyline(img, screenPolygon, true, color.RGBA{A: 255})
	}

	title := "SIDE LENGTH: " + strconv.FormatFloat(sideLength, 'g', -1, 64)
	drawTextCentered(img, title, width/2, 12*outputScale, 2*outputScale, color.RGBA{A: 255})

	file, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("create %s: %w", filename, err)
	}
	defer file.Close()
	if err := png.Encode(file, img); err != nil {
		return fmt.Errorf("write %s: %w", filename, err)
	}
	return nil
}

func plotRotation(containerSides int) (float64, float64) {
	angle := math.Pi/2 - math.Pi/float64(containerSides)
	return math.Cos(angle), math.Sin(angle)
}

func rotatePoints(points []point, cosAngle, sinAngle float64) {
	for i, p := range points {
		points[i] = rotatePoint(p, cosAngle, sinAngle)
	}
}

func rotatePoint(p point, cosAngle, sinAngle float64) point {
	return point{
		x: p.x*cosAngle - p.y*sinAngle,
		y: p.x*sinAngle + p.y*cosAngle,
	}
}

func bounds(points []point) (float64, float64, float64, float64) {
	first := points[0]
	minX, maxX := first.x, first.x
	minY, maxY := first.y, first.y
	for _, point := range points[1:] {
		minX = min(minX, point.x)
		maxX = max(maxX, point.x)
		minY = min(minY, point.y)
		maxY = max(maxY, point.y)
	}
	return minX, maxX, minY, maxY
}

func toScreenPolygon(poly []point, transform func(point) image.Point) []image.Point {
	out := make([]image.Point, len(poly))
	for i, p := range poly {
		out[i] = transform(p)
	}
	return out
}

func fillPolygon(img *image.RGBA, polygon []image.Point, c color.RGBA) {
	if len(polygon) < 3 {
		return
	}
	minY := polygon[0].Y
	maxY := polygon[0].Y
	for _, p := range polygon[1:] {
		minY = min(minY, p.Y)
		maxY = max(maxY, p.Y)
	}
	minY = max(minY, img.Bounds().Min.Y)
	maxY = min(maxY, img.Bounds().Max.Y-1)

	intersections := make([]float64, 0, len(polygon))
	for y := minY; y <= maxY; y++ {
		intersections = intersections[:0]
		scanY := float64(y) + 0.5
		for i := 0; i < len(polygon); i++ {
			a := polygon[i]
			b := polygon[(i+1)%len(polygon)]
			if a.Y == b.Y {
				continue
			}
			ay := float64(a.Y)
			by := float64(b.Y)
			if scanY < min(ay, by) || scanY >= max(ay, by) {
				continue
			}
			t := (scanY - ay) / (by - ay)
			x := float64(a.X) + t*float64(b.X-a.X)
			intersections = append(intersections, x)
		}
		sort.Float64s(intersections)
		for i := 0; i+1 < len(intersections); i += 2 {
			startX := int(math.Ceil(intersections[i]))
			endX := int(math.Floor(intersections[i+1]))
			startX = max(startX, img.Bounds().Min.X)
			endX = min(endX, img.Bounds().Max.X-1)
			for x := startX; x <= endX; x++ {
				img.SetRGBA(x, y, c)
			}
		}
	}
}

func drawPolyline(img *image.RGBA, points []image.Point, closed bool, c color.RGBA) {
	for i := 0; i+1 < len(points); i++ {
		drawLine(img, points[i], points[i+1], c)
	}
	if closed && len(points) > 1 {
		drawLine(img, points[len(points)-1], points[0], c)
	}
}

func drawLine(img *image.RGBA, a, b image.Point, c color.RGBA) {
	x0 := a.X
	y0 := a.Y
	x1 := b.X
	y1 := b.Y
	dx := absInt(x1 - x0)
	dy := -absInt(y1 - y0)
	sx := -1
	if x0 < x1 {
		sx = 1
	}
	sy := -1
	if y0 < y1 {
		sy = 1
	}
	err := dx + dy
	for {
		if image.Pt(x0, y0).In(img.Bounds()) {
			img.SetRGBA(x0, y0, c)
		}
		if x0 == x1 && y0 == y1 {
			break
		}
		e2 := 2 * err
		if e2 >= dy {
			err += dy
			x0 += sx
		}
		if e2 <= dx {
			err += dx
			y0 += sy
		}
	}
}

func drawTextCentered(img *image.RGBA, text string, centerX, y, scale int, c color.RGBA) {
	width := textWidth(text, scale)
	drawText(img, centerX-width/2, y, text, scale, c)
}

func drawText(img *image.RGBA, x, y int, text string, scale int, c color.RGBA) {
	cursor := x
	for _, r := range text {
		if r == ' ' {
			cursor += 4 * scale
			continue
		}
		glyph, ok := bitmapFont[r]
		if !ok {
			cursor += 6 * scale
			continue
		}
		for row, line := range glyph {
			for col, bit := range line {
				if bit != '1' {
					continue
				}
				for yy := 0; yy < scale; yy++ {
					for xx := 0; xx < scale; xx++ {
						px := cursor + col*scale + xx
						py := y + row*scale + yy
						if image.Pt(px, py).In(img.Bounds()) {
							img.SetRGBA(px, py, c)
						}
					}
				}
			}
		}
		cursor += 6 * scale
	}
}

func textWidth(text string, scale int) int {
	width := 0
	for _, r := range text {
		if r == ' ' {
			width += 4 * scale
		} else {
			width += 6 * scale
		}
	}
	return width
}

var bitmapFont = map[rune][]string{
	'A': {"01110", "10001", "10001", "11111", "10001", "10001", "10001"},
	'D': {"11110", "10001", "10001", "10001", "10001", "10001", "11110"},
	'E': {"11111", "10000", "10000", "11110", "10000", "10000", "11111"},
	'G': {"01111", "10000", "10000", "10011", "10001", "10001", "01111"},
	'H': {"10001", "10001", "10001", "11111", "10001", "10001", "10001"},
	'I': {"11111", "00100", "00100", "00100", "00100", "00100", "11111"},
	'L': {"10000", "10000", "10000", "10000", "10000", "10000", "11111"},
	'N': {"10001", "11001", "10101", "10011", "10001", "10001", "10001"},
	'S': {"01111", "10000", "10000", "01110", "00001", "00001", "11110"},
	'T': {"11111", "00100", "00100", "00100", "00100", "00100", "00100"},
	'0': {"01110", "10001", "10011", "10101", "11001", "10001", "01110"},
	'1': {"00100", "01100", "00100", "00100", "00100", "00100", "01110"},
	'2': {"01110", "10001", "00001", "00010", "00100", "01000", "11111"},
	'3': {"11110", "00001", "00001", "01110", "00001", "00001", "11110"},
	'4': {"00010", "00110", "01010", "10010", "11111", "00010", "00010"},
	'5': {"11111", "10000", "10000", "11110", "00001", "00001", "11110"},
	'6': {"01110", "10000", "10000", "11110", "10001", "10001", "01110"},
	'7': {"11111", "00001", "00010", "00100", "01000", "01000", "01000"},
	'8': {"01110", "10001", "10001", "01110", "10001", "10001", "01110"},
	'9': {"01110", "10001", "10001", "01111", "00001", "00001", "01110"},
	'.': {"00000", "00000", "00000", "00000", "00000", "01100", "01100"},
	':': {"00000", "01100", "01100", "00000", "01100", "01100", "00000"},
	'-': {"00000", "00000", "00000", "11111", "00000", "00000", "00000"},
	'+': {"00000", "00100", "00100", "11111", "00100", "00100", "00000"},
}

func scaleFloat64s(values []float64, multiplier float64) []float64 {
	out := make([]float64, len(values))
	for i, value := range values {
		out[i] = value * multiplier
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
	value := 0.0
	for i := range a {
		value += a[i] * b[i]
	}
	return value
}

func axpy(dst []float64, x []float64, alpha float64) {
	for i := range dst {
		dst[i] += alpha * x[i]
	}
}

func maxAbs(values []float64) float64 {
	maxValue := 0.0
	for _, value := range values {
		absValue := math.Abs(value)
		if absValue > maxValue {
			maxValue = absValue
		}
	}
	return maxValue
}

func allFinite(values []float64) bool {
	for _, value := range values {
		if !isFinite(value) {
			return false
		}
	}
	return true
}

func isFinite(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0)
}

func absInt(value int) int {
	if value < 0 {
		return -value
	}
	return value
}
