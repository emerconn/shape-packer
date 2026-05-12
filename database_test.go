package main

import "testing"

func TestPackingDocIDPolygonInPolygon(t *testing.T) {
	cfg, _ := parseArgs(testPolygonArgs(3, 4, 6))
	got := packingDocID(cfg)
	want := "3_4_in_6"
	if got != want {
		t.Fatalf("packingDocID = %q, want %q", got, want)
	}
}

func TestPackingDocIDCircleInCircle(t *testing.T) {
	cfg, _ := parseArgs(testCircleInCircleArgs(5))
	got := packingDocID(cfg)
	want := "5_0_in_0"
	if got != want {
		t.Fatalf("packingDocID = %q, want %q", got, want)
	}
}

func TestPackingDocIDPolygonInCircle(t *testing.T) {
	cfg, _ := parseArgs(testPolygonInCircleArgs(7, 3))
	got := packingDocID(cfg)
	want := "7_3_in_0"
	if got != want {
		t.Fatalf("packingDocID = %q, want %q", got, want)
	}
}

func TestPackingDocIDCircleInPolygon(t *testing.T) {
	cfg, _ := parseArgs(testCircleInPolygonArgs(4, 8))
	got := packingDocID(cfg)
	want := "4_0_in_8"
	if got != want {
		t.Fatalf("packingDocID = %q, want %q", got, want)
	}
}

func TestInnerSidesStringPolygon(t *testing.T) {
	cfg, _ := parseArgs(testPolygonArgs(1, 5, 6))
	if got := innerSidesString(cfg); got != "5" {
		t.Fatalf("innerSidesString = %q, want %q", got, "5")
	}
}

func TestInnerSidesStringCircle(t *testing.T) {
	cfg, _ := parseArgs(testCircleInCircleArgs(3))
	if got := innerSidesString(cfg); got != "0" {
		t.Fatalf("innerSidesString = %q, want %q", got, "0")
	}
}

func TestOuterSidesStringPolygon(t *testing.T) {
	cfg, _ := parseArgs(testPolygonArgs(1, 3, 8))
	if got := outerSidesString(cfg); got != "8" {
		t.Fatalf("outerSidesString = %q, want %q", got, "8")
	}
}

func TestOuterSidesStringCircle(t *testing.T) {
	cfg, _ := parseArgs(testPolygonInCircleArgs(2, 3))
	if got := outerSidesString(cfg); got != "0" {
		t.Fatalf("outerSidesString = %q, want %q", got, "0")
	}
}
