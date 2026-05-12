package main

import (
	"image"
	"image/color"
	"math"
	"os"
	"path/filepath"
	"testing"
)

func TestPlotRotationAlignsCommonContainerEdges(t *testing.T) {
	cfg, err := parseArgs(testPolygonArgs(1, 3, 3, "--attempts=1"))
	if err != nil {
		t.Fatalf("parseArgs returned error: %v", err)
	}
	triangle := rotatedContainer(cfg)
	if !hasHorizontalEdge(triangle) {
		t.Fatalf("rotated triangle container does not have a horizontal side: %#v", triangle)
	}

	cfg, err = parseArgs(testPolygonArgs(1, 3, 4, "--attempts=1"))
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

func TestPlotRotationCircleContainer(t *testing.T) {
	cfg, err := parseArgs(testCircleInCircleArgs(1, "--attempts=1"))
	if err != nil {
		t.Fatalf("parseArgs returned error: %v", err)
	}
	cosA, sinA := plotRotation(cfg)
	if math.Abs(cosA-1) > 1e-12 || math.Abs(sinA) > 1e-12 {
		t.Fatalf("plotRotation for circle = (%g, %g), want (1, 0)", cosA, sinA)
	}
}

func TestAbsInt(t *testing.T) {
	if v := absInt(5); v != 5 {
		t.Fatalf("absInt(5) = %d, want 5", v)
	}
	if v := absInt(-5); v != 5 {
		t.Fatalf("absInt(-5) = %d, want 5", v)
	}
	if v := absInt(0); v != 0 {
		t.Fatalf("absInt(0) = %d, want 0", v)
	}
}

func TestTextWidth(t *testing.T) {
	if v := textWidth("A", 1); v != 6 {
		t.Fatalf("textWidth(\"A\", 1) = %d, want 6", v)
	}
	if v := textWidth("AB", 1); v != 12 {
		t.Fatalf("textWidth(\"AB\", 1) = %d, want 12", v)
	}
	if v := textWidth("A B", 1); v != 16 {
		t.Fatalf("textWidth(\"A B\", 1) = %d, want 16", v) // 6 + 4 + 6
	}
	if v := textWidth("A", 2); v != 12 {
		t.Fatalf("textWidth(\"A\", 2) = %d, want 12", v)
	}
	if v := textWidth("", 1); v != 0 {
		t.Fatalf("textWidth(\"\", 1) = %d, want 0", v)
	}
}

func TestDrawText(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 200, 50))
	drawText(img, 0, 0, "A", 1, color.RGBA{A: 255})
	// Verify some pixels were set
	found := false
	for y := 0; y < 50; y++ {
		for x := 0; x < 200; x++ {
			if img.RGBAAt(x, y).A != 0 {
				found = true
				break
			}
		}
		if found {
			break
		}
	}
	if !found {
		t.Fatal("drawText set no pixels")
	}
}

func TestDrawTextUnknownGlyph(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 200, 50))
	drawText(img, 0, 0, "Z", 1, color.RGBA{A: 255})
	// Should not panic, unknown glyphs are skipped (cursor still advances)
}

func TestDrawTextCentered(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 200, 50))
	drawTextCentered(img, "HI", 100, 5, 1, color.RGBA{A: 255})
	// Verify some pixels near center were set
	found := false
	for y := 5; y < 20; y++ {
		for x := 80; x < 120; x++ {
			if img.RGBAAt(x, y).A != 0 {
				found = true
				break
			}
		}
		if found {
			break
		}
	}
	if !found {
		t.Fatal("drawTextCentered set no pixels near center")
	}
}

func TestDrawLine(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 10, 10))
	drawLine(img, image.Point{X: 0, Y: 0}, image.Point{X: 9, Y: 9}, color.RGBA{R: 255, A: 255})
	if img.RGBAAt(0, 0).R != 255 {
		t.Fatal("start pixel not drawn")
	}
	if img.RGBAAt(9, 9).R != 255 {
		t.Fatal("end pixel not drawn")
	}
}

func TestDrawLineOutOfBounds(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 10, 10))
	drawLine(img, image.Point{X: -5, Y: -5}, image.Point{X: 15, Y: 15}, color.RGBA{A: 255})
	// Should not panic, out-of-bounds pixels are clipped
}

func TestDrawLineHorizontal(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 10, 10))
	drawLine(img, image.Point{X: 0, Y: 5}, image.Point{X: 9, Y: 5}, color.RGBA{A: 255})
	for x := 0; x <= 9; x++ {
		if img.RGBAAt(x, 5).A == 0 {
			t.Fatalf("pixel (%d, 5) not drawn", x)
		}
	}
}

func TestDrawLineVertical(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 10, 10))
	drawLine(img, image.Point{X: 5, Y: 0}, image.Point{X: 5, Y: 9}, color.RGBA{A: 255})
	for y := 0; y <= 9; y++ {
		if img.RGBAAt(5, y).A == 0 {
			t.Fatalf("pixel (5, %d) not drawn", y)
		}
	}
}

func TestDrawPolyline(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 20, 20))
	points := []image.Point{{X: 0, Y: 0}, {X: 10, Y: 0}, {X: 10, Y: 10}}
	drawPolyline(img, points, false, color.RGBA{A: 255})
	// First segment: horizontal line from (0,0) to (10,0)
	if img.RGBAAt(5, 0).A == 0 {
		t.Fatal("first segment not drawn")
	}
	// Not closed, so no line from (10,10) back to (0,0)
	if img.RGBAAt(0, 5).A != 0 {
		t.Fatal("unexpected pixel for unclosed polyline")
	}
}

func TestDrawPolylineClosed(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 20, 20))
	points := []image.Point{{X: 2, Y: 2}, {X: 8, Y: 2}, {X: 8, Y: 8}}
	drawPolyline(img, points, true, color.RGBA{A: 255})
	// Closed: should also draw from (8,8) back to (2,2)
	if img.RGBAAt(2, 2).A == 0 {
		t.Fatal("starting pixel not drawn")
	}
}

func TestFillPolygon(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 20, 20))
	polygon := []image.Point{{X: 5, Y: 5}, {X: 15, Y: 5}, {X: 15, Y: 15}, {X: 5, Y: 15}}
	fillPolygon(img, polygon, color.RGBA{R: 255, G: 0, B: 0, A: 255})
	// Center of the rectangle should be filled
	c := img.RGBAAt(10, 10)
	if c.R != 255 || c.A != 255 {
		t.Fatalf("center pixel = %v, want red", c)
	}
}

func TestFillPolygonClippedToImageBounds(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 10, 10))
	polygon := []image.Point{{X: -5, Y: -5}, {X: 15, Y: -5}, {X: 15, Y: 15}, {X: -5, Y: 15}}
	fillPolygon(img, polygon, color.RGBA{R: 255, A: 255})
	// Should not panic and should fill within bounds
	c := img.RGBAAt(5, 5)
	if c.R != 255 {
		t.Fatal("clipped polygon should fill center")
	}
}

func TestFillPolygonLessThanThreePoints(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 10, 10))
	fillPolygon(img, []image.Point{{X: 0, Y: 0}, {X: 5, Y: 5}}, color.RGBA{A: 255})
	// Should not panic, early return
}

func TestToScreenPolygon(t *testing.T) {
	points := []point{{x: 1, y: 2}, {x: 3, y: 4}}
	transform := func(p point) image.Point {
		return image.Point{X: int(p.x * 10), Y: int(p.y * 10)}
	}
	result := toScreenPolygon(points, transform)
	if len(result) != 2 {
		t.Fatalf("len = %d, want 2", len(result))
	}
	if result[0].X != 10 || result[0].Y != 20 {
		t.Fatalf("result[0] = %v, want (10, 20)", result[0])
	}
	if result[1].X != 30 || result[1].Y != 40 {
		t.Fatalf("result[1] = %v, want (30, 40)", result[1])
	}
}

func TestSavePlot(t *testing.T) {
	cfg, _ := parseArgs(testPolygonArgs(3, 4, 6, "--attempts=1"))
	values := []float64{0.5, 0.5, 0.1, -0.5, 0.3, 0.5, 0.1, -0.4, 0.8}
	dir := t.TempDir()
	filename := filepath.Join(dir, "test.png")

	err := writePlotToFile(filename, cfg, 2.0, values, "side length", 1.5, 1)
	if err != nil {
		t.Fatalf("savePlot error: %v", err)
	}
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		t.Fatal("output file not created")
	}
}

func TestSavePlotCircleInCircle(t *testing.T) {
	cfg, _ := parseArgs(testCircleInCircleArgs(3, "--attempts=1"))
	values := []float64{-0.5, -0.5, 0.5, 0.5, 0.0, -0.3}
	dir := t.TempDir()
	filename := filepath.Join(dir, "test.png")

	err := writePlotToFile(filename, cfg, 3.0, values, "size", 2.0, 1)
	if err != nil {
		t.Fatalf("savePlot error: %v", err)
	}
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		t.Fatal("output file not created")
	}
}

func TestSavePlotMultipleScales(t *testing.T) {
	cfg, _ := parseArgs(testPolygonArgs(2, 3, 4, "--attempts=1"))
	values := []float64{0.3, 0.3, 0.5, -0.3, -0.3, 1.0}
	dir := t.TempDir()

	for _, scale := range outputScales {
		filename := filepath.Join(dir, "test.png")
		err := writePlotToFile(filename, cfg, 2.0, values, "side length", 1.5, scale)
		if err != nil {
			t.Fatalf("savePlot scale=%d error: %v", scale, err)
		}
	}
}

func TestSavePlotCreatesDirectory(t *testing.T) {
	cfg, _ := parseArgs(testPolygonArgs(1, 3, 4, "--attempts=1"))
	values := []float64{0, 0, 0}
	dir := t.TempDir()
	nestedDir := filepath.Join(dir, "nested", "dir")
	filename := filepath.Join(nestedDir, "test.png")

	err := writePlotToFile(filename, cfg, 2.0, values, "side length", 1.5, 1)
	if err != nil {
		t.Fatalf("savePlot error: %v", err)
	}
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		t.Fatal("output file not created in nested directory")
	}
}

func rotatedContainer(cfg *config) []point {
	cosAngle, sinAngle := plotRotation(cfg)
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
