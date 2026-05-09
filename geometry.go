package main

import "math"

type point struct {
	x, y float64
}

type gridCell struct {
	x, y int
}

func transformPolygonAndVectors(x, y, angle float64, cfg *config, containerLimit float64, polygonOut, vectorOut []point) float64 {
	sinAngle, cosAngle := math.Sincos(angle)
	penalty := 0.0

	for i := range cfg.unitPolygonVertices {
		vertex := cfg.unitPolygonVertices[i]
		px := x + vertex.x*cosAngle - vertex.y*sinAngle
		py := y + vertex.x*sinAngle + vertex.y*cosAngle
		polygonOut[i] = point{x: px, y: py}

		for _, vector := range cfg.unitContainerVectors {
			distance := px*vector.x + py*vector.y
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
			x: x + vertex.x*cosAngle - vertex.y*sinAngle,
			y: y + vertex.x*sinAngle + vertex.y*cosAngle,
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

func rotatePoint(p point, cosAngle, sinAngle float64) point {
	return point{
		x: p.x*cosAngle - p.y*sinAngle,
		y: p.x*sinAngle + p.y*cosAngle,
	}
}

func rotatePoints(points []point, cosAngle, sinAngle float64) {
	for i, p := range points {
		points[i] = rotatePoint(p, cosAngle, sinAngle)
	}
}

func plotRotation(cfg *config) (float64, float64) {
	if cfg.outerIsCircle() {
		return 1.0, 0.0
	}
	angle := math.Pi/2 - math.Pi/float64(cfg.containerSides)
	return math.Cos(angle), math.Sin(angle)
}

func projectPolygon(vertices []point, axisX, axisY float64) (float64, float64) {
	d := vertices[0].x*axisX + vertices[0].y*axisY
	minVal, maxVal := d, d
	for _, v := range vertices[1:] {
		d = v.x*axisX + v.y*axisY
		if d < minVal {
			minVal = d
		}
		if d > maxVal {
			maxVal = d
		}
	}
	return minVal, maxVal
}

func intervalOverlap(minA, maxA, minB, maxB float64) float64 {
	return min(maxA, maxB) - max(minA, minB)
}

func circleInPolygonPenalty(cx, cy, radius float64, containerVectors []point, containerLimit float64) float64 {
	penalty := 0.0
	for _, v := range containerVectors {
		projection := cx*v.x + cy*v.y
		if projection+radius > containerLimit {
			diff := projection + radius - containerLimit
			penalty += diff * diff
		}
	}
	return penalty
}

func circleInCirclePenalty(cx, cy, radius float64, containerRadius float64) float64 {
	dist := math.Sqrt(cx*cx + cy*cy)
	if dist+radius > containerRadius {
		diff := dist + radius - containerRadius
		return diff * diff
	}
	return 0
}

func polygonInCirclePenalty(vertices []point, containerRadius float64) float64 {
	penalty := 0.0
	for _, v := range vertices {
		dist := math.Sqrt(v.x*v.x + v.y*v.y)
		if dist > containerRadius {
			diff := dist - containerRadius
			penalty += diff * diff
		}
	}
	return penalty
}

func circlePoints(cx, cy, radius float64, n int) []point {
	points := make([]point, n)
	for i := range n {
		angle := 2 * math.Pi * float64(i) / float64(n)
		points[i] = point{x: cx + radius*math.Cos(angle), y: cy + radius*math.Sin(angle)}
	}
	return points
}

func bounds(points []point) (float64, float64, float64, float64) {
	minX, maxX := points[0].x, points[0].x
	minY, maxY := points[0].y, points[0].y
	for _, p := range points[1:] {
		minX = min(minX, p.x)
		maxX = max(maxX, p.x)
		minY = min(minY, p.y)
		maxY = max(maxY, p.y)
	}
	return minX, maxX, minY, maxY
}
