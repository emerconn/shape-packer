package main

import "testing"

func TestGcsURLWithFullPath(t *testing.T) {
	got := gcsURL("my-bucket", "21_3_in_0_res1024.png")
	want := "https://storage.googleapis.com/my-bucket/21_3_in_0_res1024.png"
	if got != want {
		t.Fatalf("gcsURL = %q, want %q", got, want)
	}
}

func TestGcsURLPlainFilename(t *testing.T) {
	got := gcsURL("polygon-picker", "5_0_in_0_res1.png")
	want := "https://storage.googleapis.com/polygon-picker/5_0_in_0_res1.png"
	if got != want {
		t.Fatalf("gcsURL = %q, want %q", got, want)
	}
}
