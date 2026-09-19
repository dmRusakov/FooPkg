package tilewriter_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"FooPkg/tile_gen/tilewriter"
	"github.com/stretchr/testify/require"
)

func TestGenerateSmoke(t *testing.T) {
	// Write a minimal GeoJSON FeatureCollection.
	gj := map[string]interface{}{
		"type": "FeatureCollection",
		"features": []interface{}{
			map[string]interface{}{
				"type": "Feature",
				"geometry": map[string]interface{}{
					"type":        "Point",
					"coordinates": []float64{13.4050, 52.5200},
				},
				"properties": map[string]interface{}{
					"name": "Berlin",
					"pop":  3769000,
				},
			},
			map[string]interface{}{
				"type": "Feature",
				"geometry": map[string]interface{}{
					"type":        "Point",
					"coordinates": []float64{2.3522, 48.8566},
				},
				"properties": map[string]interface{}{
					"name": "Paris",
					"pop":  2161000,
				},
			},
		},
	}

	dir := t.TempDir()
	inputPath := filepath.Join(dir, "cities.geojson")
	outputPath := filepath.Join(dir, "cities.mbtiles")

	data, _ := json.Marshal(gj)
	require.NoError(t, os.WriteFile(inputPath, data, 0644))

	cfg := tilewriter.DefaultConfig()
	cfg.MaxZoom = 5
	cfg.MinZoom = 0

	err := tilewriter.Generate(context.Background(), inputPath, outputPath, cfg)
	require.NoError(t, err)

	info, err := os.Stat(outputPath)
	require.NoError(t, err)
	require.Greater(t, info.Size(), int64(0))
}

func TestDropByRate(t *testing.T) {
	// Ensure lower zoom levels produce fewer tiles by not panicking.
	gj := map[string]interface{}{
		"type":     "FeatureCollection",
		"features": makePoints(200),
	}

	dir := t.TempDir()
	inputPath := filepath.Join(dir, "pts.geojson")
	outputPath := filepath.Join(dir, "pts.mbtiles")

	data, _ := json.Marshal(gj)
	require.NoError(t, os.WriteFile(inputPath, data, 0644))

	cfg := tilewriter.DefaultConfig()
	cfg.MaxZoom = 3
	cfg.MinZoom = 0
	cfg.DropRate = 2.0

	require.NoError(t, tilewriter.Generate(context.Background(), inputPath, outputPath, cfg))
}

func makePoints(n int) []interface{} {
	pts := make([]interface{}, n)
	for i := 0; i < n; i++ {
		lon := -180.0 + float64(i)*360.0/float64(n)
		pts[i] = map[string]interface{}{
			"type": "Feature",
			"geometry": map[string]interface{}{
				"type":        "Point",
				"coordinates": []float64{lon, 0},
			},
			"properties": map[string]interface{}{"i": i},
		}
	}
	return pts
}
