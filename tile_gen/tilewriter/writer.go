package tilewriter

import (
	"context"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"

	"FooPkg/tile_gen/geojson"
	"FooPkg/tile_gen/mbtiles"
	"FooPkg/tile_gen/projection"
	"FooPkg/tile_gen/serial"
)

// Generate reads GeoJSON from inputPath and writes tiles to outputPath (mbtiles).
func Generate(ctx context.Context, inputPath, outputPath string, cfg Config) error {
	// Open input.
	f, err := os.Open(inputPath)
	if err != nil {
		return fmt.Errorf("open input: %w", err)
	}
	defer f.Close()

	// Derive layer name from input filename if not overridden.
	if cfg.LayerName == "" {
		base := filepath.Base(inputPath)
		cfg.LayerName = strings.TrimSuffix(base, filepath.Ext(base))
	}

	// Parse GeoJSON.
	features, err := geojson.ParseReader(f, cfg.LayerName)
	if err != nil {
		return fmt.Errorf("parse geojson: %w", err)
	}
	if len(features) == 0 {
		return nil
	}

	// Apply zoom properties from tippecanoe:minzoom / maxzoom attributes.
	for i := range features {
		features[i].ApplyZoomProperties()
	}

	// Compute spatial index for each feature.
	indexFeatures(features, cfg.IndexType)

	// Compute bounding box over all features for metadata.
	bounds := computeBounds(features)

	// Open output database.
	db, err := mbtiles.Open(outputPath)
	if err != nil {
		return fmt.Errorf("open mbtiles: %w", err)
	}
	defer db.Close()

	// Write tiles.
	if err := TraverseZooms(features, db, &cfg); err != nil {
		return err
	}

	// Write metadata.
	meta := mbtiles.Metadata{
		Name:        cfg.Name,
		Description: cfg.Description,
		Attribution: cfg.Attribution,
		Version:     "2",
		Type:        "overlay",
		MinZoom:     cfg.MinZoom,
		MaxZoom:     cfg.MaxZoom,
		Bounds:      bounds,
		Center:      boundsCenter(bounds, cfg.MaxZoom),
	}
	if meta.Name == "" {
		meta.Name = cfg.LayerName
	}
	return db.WriteMetadata(meta)
}

func indexFeatures(features []serial.Feature, indexType IndexType) {
	for i := range features {
		f := &features[i]
		cx := (f.BBox.MinX + f.BBox.MaxX) / 2
		cy := (f.BBox.MinY + f.BBox.MaxY) / 2
		switch indexType {
		case IndexQuadkey:
			f.Index = projection.QuadkeyIndex(cx, cy)
		default:
			f.Index = projection.HilbertIndexXY(cx, cy)
		}
		f.Seq = int64(i)
	}
}

func computeBounds(features []serial.Feature) string {
	if len(features) == 0 {
		return "-180,-85.0511,180,85.0511"
	}
	minX := features[0].BBox.MinX
	minY := features[0].BBox.MinY
	maxX := features[0].BBox.MaxX
	maxY := features[0].BBox.MaxY
	for _, f := range features[1:] {
		if f.BBox.MinX < minX {
			minX = f.BBox.MinX
		}
		if f.BBox.MinY < minY {
			minY = f.BBox.MinY
		}
		if f.BBox.MaxX > maxX {
			maxX = f.BBox.MaxX
		}
		if f.BBox.MaxY > maxY {
			maxY = f.BBox.MaxY
		}
	}
	lon0, lat0 := projection.TileToLonLat(minX, maxY)
	lon1, lat1 := projection.TileToLonLat(maxX, minY)
	return fmt.Sprintf("%.6f,%.6f,%.6f,%.6f", lon0, lat0, lon1, lat1)
}

func boundsCenter(bounds string, zoom int) string {
	var lon0, lat0, lon1, lat1 float64
	fmt.Sscanf(bounds, "%f,%f,%f,%f", &lon0, &lat0, &lon1, &lat1)
	cLon := (lon0 + lon1) / 2
	cLat := (lat0 + lat1) / 2
	// Clamp to valid range.
	cLon = math.Max(-180, math.Min(180, cLon))
	cLat = math.Max(-85.0511, math.Min(85.0511, cLat))
	return fmt.Sprintf("%.6f,%.6f,%d", cLon, cLat, zoom)
}
