package main

import (
	"math"
	"testing"
)

func TestRotatePoints(t *testing.T) {
	points := []point{{x: 1, y: 0}, {x: 0, y: 1}}
	angle := math.Pi / 2
	sinA, cosA := math.Sincos(angle)
	rotatePoints(points, cosA, sinA)

	if math.Abs(points[0].x) > 1e-10 || math.Abs(points[0].y-1) > 1e-10 {
		t.Fatalf("points[0] = %v, want (0, 1)", points[0])
	}
	if math.Abs(points[1].x-(-1)) > 1e-12 || math.Abs(points[1].y) > 1e-12 {
		t.Fatalf("points[1] = %v, want (-1, 0)", points[1])
	}
}

func TestRotatePointsIdentity(t *testing.T) {
	original := []point{{x: 3.5, y: -2.1}, {x: -1, y: 4}}
	points := make([]point, len(original))
	copy(points, original)
	rotatePoints(points, 1, 0) // cos(0)=1, sin(0)=0

	for i, p := range points {
		if math.Abs(p.x-original[i].x) > 1e-12 || math.Abs(p.y-original[i].y) > 1e-12 {
			t.Fatalf("points[%d] = %v, want %v", i, p, original[i])
		}
	}
}

func TestBoundsSinglePoint(t *testing.T) {
	minX, maxX, minY, maxY := bounds([]point{{x: 5, y: -3}})
	if minX != 5 || maxX != 5 || minY != -3 || maxY != -3 {
		t.Fatalf("bounds = (%g, %g, %g, %g), want (5, 5, -3, -3)", minX, maxX, minY, maxY)
	}
}

func TestBoundsMultiplePoints(t *testing.T) {
	minX, maxX, minY, maxY := bounds([]point{
		{x: 1, y: 2}, {x: -3, y: 5}, {x: 4, y: -1},
	})
	if minX != -3 || maxX != 4 || minY != -1 || maxY != 5 {
		t.Fatalf("bounds = (%g, %g, %g, %g), want (-3, 4, -1, 5)", minX, maxX, minY, maxY)
	}
}

func TestBoundsNegativeCoordinates(t *testing.T) {
	minX, maxX, minY, maxY := bounds([]point{
		{x: -10, y: -20}, {x: -1, y: -5},
	})
	if minX != -10 || maxX != -1 || minY != -20 || maxY != -5 {
		t.Fatalf("bounds = (%g, %g, %g, %g), want (-10, -1, -20, -5)", minX, maxX, minY, maxY)
	}
}
