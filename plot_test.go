package main

import (
	"math"
	"testing"
)

func TestPlotRotationAlignsCommonContainerEdges(t *testing.T) {
	cfg, err := parseArgs([]string{"1", "3", "3", "--attempts", "1"})
	if err != nil {
		t.Fatalf("parseArgs returned error: %v", err)
	}
	triangle := rotatedContainer(cfg)
	if !hasHorizontalEdge(triangle) {
		t.Fatalf("rotated triangle container does not have a horizontal side: %#v", triangle)
	}

	cfg, err = parseArgs([]string{"1", "3", "4", "--attempts", "1"})
	if err != nil {
		t.Fatalf("parseArgs returned error: %v", err)
	}
	square := rotatedContainer(cfg)
	for i := range square {
		a := square[i]
		b := square[(i+1)%len(square)]
		if math.Abs(a.x-b.x) > 1e-12 && math.Abs(a.y-b.y) > 1e-12 {
			t.Fatalf("rotated square edge %d is not horizontal or vertical: %#v -> %#v", i, a, b)
		}
	}
}

func rotatedContainer(cfg *config) []point {
	cosAngle, sinAngle := plotRotation(cfg.containerSides)
	vertices := make([]point, cfg.containerSides)
	for i, vertex := range cfg.unitContainerVertices {
		vertices[i] = rotatePoint(vertex, cosAngle, sinAngle)
	}
	return vertices
}

func hasHorizontalEdge(vertices []point) bool {
	for i := range vertices {
		if math.Abs(vertices[i].y-vertices[(i+1)%len(vertices)].y) <= 1e-12 {
			return true
		}
	}
	return false
}
