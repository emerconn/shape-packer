package main

import (
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
)

var outputScales = [...]int{1, 2, 4, 6}

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
	for i := range cfg.innerPolygons {
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
	defer func() { err = errors.Join(err, file.Close()) }()
	if err := png.Encode(file, img); err != nil {
		return fmt.Errorf("write %s: %w", filename, err)
	}
	return nil
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
		for i := range polygon {
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
	x0, y0 := a.X, a.Y
	x1, y1 := b.X, b.Y
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
	w := textWidth(text, scale)
	drawText(img, centerX-w/2, y, text, scale, c)
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
				for yy := range scale {
					for xx := range scale {
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
	w := 0
	for _, r := range text {
		if r == ' ' {
			w += 4 * scale
		} else {
			w += 6 * scale
		}
	}
	return w
}

func absInt(value int) int {
	if value < 0 {
		return -value
	}
	return value
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
