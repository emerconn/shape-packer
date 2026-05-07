package main

import "math"

const (
	spatialGridThreshold = 96
	spatialGridCellSize  = 2.0
)

type evaluator struct {
	cfg              *config
	polys            []point
	vectors          []point
	cells            []gridCell
	nextInCell       []int
	cellHeads        map[gridCell]int
	usedCells        []gridCell
	polygonPenalties []float64
	pairPenalties    []float64
}

type packingObjective struct {
	eval *evaluator
	side float64
}

func newEvaluator(cfg *config) *evaluator {
	return &evaluator{
		cfg:              cfg,
		polys:            make([]point, cfg.innerPolygons*cfg.innerSides),
		vectors:          make([]point, cfg.innerPolygons*cfg.innerSides),
		cells:            make([]gridCell, cfg.innerPolygons),
		nextInCell:       make([]int, cfg.innerPolygons),
		cellHeads:        make(map[gridCell]int, cfg.innerPolygons),
		usedCells:        make([]gridCell, 0, cfg.innerPolygons),
		polygonPenalties: make([]float64, cfg.innerPolygons),
	}
}

func newPackingObjective(cfg *config, side float64) *packingObjective {
	return &packingObjective{
		eval: newEvaluator(cfg),
		side: side,
	}
}

func (o *packingObjective) value(values []float64) float64 {
	return o.eval.value(values, o.side)
}

func (o *packingObjective) gradient(values []float64, f0 float64, gradient []float64, maxEvals int) int {
	return o.eval.finiteDifferenceGradient(values, o.side, f0, gradient, maxEvals)
}

func (e *evaluator) value(values []float64, side float64) float64 {
	cfg := e.cfg
	innerSides := cfg.innerSides
	containerLimit := cfg.unitContainerApothem * side

	penalty := 0.0
	for i := range cfg.innerPolygons {
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
		for i := range cfg.innerPolygons {
			for j := i + 1; j < cfg.innerPolygons; j++ {
				penalty += e.pairPenalty(values, i, j)
			}
		}
	}

	return penalty
}

func (e *evaluator) finiteDifferenceGradient(values []float64, side float64, f0 float64, gradient []float64, maxEvals int) int {
	if e.cfg.innerPolygons >= spatialGridThreshold {
		return finiteDifferenceGradient(func(x []float64) float64 {
			return e.value(x, side)
		}, values, f0, gradient, maxEvals)
	}
	return e.incrementalFiniteDifferenceGradient(values, side, f0, gradient, maxEvals)
}

func (e *evaluator) incrementalFiniteDifferenceGradient(values []float64, side float64, f0 float64, gradient []float64, maxEvals int) int {
	if maxEvals <= 0 {
		for i := range gradient {
			gradient[i] = 0
		}
		return 0
	}

	e.valueWithPairPenalties(values, side)

	cfg := e.cfg
	innerSides := cfg.innerSides
	containerLimit := cfg.unitContainerApothem * side
	evals := 0
	for i := range values {
		if evals >= maxEvals {
			for j := i; j < len(values); j++ {
				gradient[j] = 0
			}
			break
		}

		polygonIndex := i / 3
		valueBase := polygonIndex * 3
		polygonBase := polygonIndex * innerSides
		polygon := e.polys[polygonBase : polygonBase+innerSides]
		vectors := e.vectors[polygonBase : polygonBase+innerSides]

		original := values[i]
		values[i] = original + lbfgsGradientEps
		polygonPenalty := transformPolygonAndVectors(
			values[valueBase],
			values[valueBase+1],
			values[valueBase+2],
			cfg,
			containerLimit,
			polygon,
			vectors,
		)

		delta := polygonPenalty - e.polygonPenalties[polygonIndex]
		for other := range cfg.innerPolygons {
			if other == polygonIndex {
				continue
			}
			a := polygonIndex
			b := other
			if b < a {
				a, b = b, a
			}
			delta += e.pairPenalty(values, a, b) - e.pairPenalties[pairPenaltyIndex(a, b, cfg.innerPolygons)]
		}

		values[i] = original
		transformPolygonAndVectors(
			values[valueBase],
			values[valueBase+1],
			values[valueBase+2],
			cfg,
			containerLimit,
			polygon,
			vectors,
		)

		f1 := f0 + delta
		if isFinite(f1) {
			gradient[i] = (f1 - f0) / lbfgsGradientEps
		} else {
			gradient[i] = 0
		}
		evals++
	}
	return evals
}

func (e *evaluator) valueWithPairPenalties(values []float64, side float64) float64 {
	cfg := e.cfg
	innerSides := cfg.innerSides
	containerLimit := cfg.unitContainerApothem * side
	penalty := 0.0

	for i := range cfg.innerPolygons {
		base := i * innerSides
		valueBase := i * 3
		polygonPenalty := transformPolygonAndVectors(
			values[valueBase],
			values[valueBase+1],
			values[valueBase+2],
			cfg,
			containerLimit,
			e.polys[base:base+innerSides],
			e.vectors[base:base+innerSides],
		)
		e.polygonPenalties[i] = polygonPenalty
		penalty += polygonPenalty
	}

	e.ensurePairPenalties()
	for i := range cfg.innerPolygons {
		for j := i + 1; j < cfg.innerPolygons; j++ {
			pp := e.pairPenalty(values, i, j)
			e.pairPenalties[pairPenaltyIndex(i, j, cfg.innerPolygons)] = pp
			penalty += pp
		}
	}
	return penalty
}

func (e *evaluator) ensurePairPenalties() {
	count := e.cfg.innerPolygons * (e.cfg.innerPolygons - 1) / 2
	if cap(e.pairPenalties) < count {
		e.pairPenalties = make([]float64, count)
		return
	}
	e.pairPenalties = e.pairPenalties[:count]
}

func pairPenaltyIndex(i, j, count int) int {
	return i*count - i*(i+1)/2 + j - i - 1
}

// pairPenalty computes the squared minimum-overlap penalty between polygons i
// and j using SAT. Returns 0 if they do not collide.
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
	for axis := range innerSides {
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
	for axis := range innerSides {
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

func (e *evaluator) spatialCollisionPenalty(values []float64) float64 {
	e.buildSpatialGrid(values)

	penalty := 0.0
	for i := range e.cfg.innerPolygons {
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

	for i := range e.cfg.innerPolygons {
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
