package main

import (
	"context"
	"fmt"

	"cloud.google.com/go/storage"
)

func newGCSClient(ctx context.Context) (*storage.Client, error) {
	return storage.NewClient(ctx)
}

func uploadPlot(ctx context.Context, client *storage.Client, bucket, objectName string, cfg *config, side float64, values []float64, sizeLabel string, sizeValue float64, outputScale int) (string, error) {
	writer := client.Bucket(bucket).Object(objectName).NewWriter(ctx)
	writer.ContentType = "image/png"
	writer.CacheControl = "public, max-age=31536000"

	if err := savePlot(writer, cfg, side, values, sizeLabel, sizeValue, outputScale); err != nil {
		_ = writer.Close()
		return "", fmt.Errorf("upload %s: %w", objectName, err)
	}
	if err := writer.Close(); err != nil {
		return "", fmt.Errorf("close writer for %s: %w", objectName, err)
	}

	return "https://storage.googleapis.com/" + bucket + "/" + objectName, nil
}

func gcsURL(bucket, filePath string) string {
	return "https://storage.googleapis.com/" + bucket + "/" + filePath
}
