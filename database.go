package main

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"cloud.google.com/go/firestore"
)

const (
	packingsCollection = "packings"
	versionsCollection = "versions"
)

type RunRecord struct {
	InnerCount int               `firestore:"inner_count"`
	InnerSides int               `firestore:"inner_sides"`
	OuterSides int               `firestore:"outer_sides"`
	InnerType  string            `firestore:"inner_type"`
	OuterType  string            `firestore:"outer_type"`
	Attempts   int               `firestore:"attempts"`
	Tolerance  float64           `firestore:"tolerance"`
	FinalStep  float64           `firestore:"final_step"`
	SizeLabel  string            `firestore:"size_label"`
	SizeValue  float64           `firestore:"size_value"`
	ImageURLs  map[string]string `firestore:"image_urls"`
	CreatedAt  time.Time         `firestore:"created_at"`
}

type PackingSummary struct {
	InnerCount   int       `firestore:"inner_count"`
	InnerSides   int       `firestore:"inner_sides"`
	OuterSides   int       `firestore:"outer_sides"`
	InnerType    string    `firestore:"inner_type"`
	OuterType    string    `firestore:"outer_type"`
	SizeLabel    string    `firestore:"size_label"`
	VersionCount int       `firestore:"version_count"`
	UpdatedAt    time.Time `firestore:"updated_at"`
}

func packingDocID(cfg *config) string {
	return fmt.Sprintf("%d_%s_in_%s", cfg.innerCount, innerSidesString(cfg), outerSidesString(cfg))
}

func innerSidesString(cfg *config) string {
	if cfg.innerIsPolygon() {
		return strconv.Itoa(cfg.innerSides)
	}
	return "0"
}

func outerSidesString(cfg *config) string {
	if cfg.outerIsPolygon() {
		return strconv.Itoa(cfg.containerSides)
	}
	return "0"
}

func newFirestoreClient(ctx context.Context, projectID, databaseID string) (*firestore.Client, error) {
	return firestore.NewClientWithDatabase(ctx, projectID, databaseID)
}

func saveRun(ctx context.Context, client *firestore.Client, cfg *config, sizeLabel string, sizeValue float64, imageURLs map[string]string) error {
	docID := packingDocID(cfg)
	packingRef := client.Collection(packingsCollection).Doc(docID)

	now := time.Now().UTC()

	record := RunRecord{
		InnerCount: cfg.innerCount,
		InnerSides: cfg.innerSides,
		OuterSides: cfg.containerSides,
		InnerType:  cfg.innerType,
		OuterType:  cfg.outerType,
		Attempts:   cfg.attempts,
		Tolerance:  cfg.penaltyTolerance,
		FinalStep:  cfg.finalStepSize,
		SizeLabel:  sizeLabel,
		SizeValue:  sizeValue,
		ImageURLs:  imageURLs,
		CreatedAt:  now,
	}

	err := client.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		var versionCount int
		snap, err := tx.Get(packingRef)
		if err == nil {
			var summary PackingSummary
			if err := snap.DataTo(&summary); err == nil {
				versionCount = summary.VersionCount
			}
		}
		versionCount++

		versionRef := packingRef.Collection(versionsCollection).NewDoc()
		if err := tx.Set(versionRef, record); err != nil {
			return fmt.Errorf("set version: %w", err)
		}

		summary := PackingSummary{
			InnerCount:   cfg.innerCount,
			InnerSides:   cfg.innerSides,
			OuterSides:   cfg.containerSides,
			InnerType:    cfg.innerType,
			OuterType:    cfg.outerType,
			SizeLabel:    sizeLabel,
			VersionCount: versionCount,
			UpdatedAt:    now,
		}
		return tx.Set(packingRef, summary)
	})
	if err != nil {
		return fmt.Errorf("firestore transaction: %w", err)
	}

	return nil
}
