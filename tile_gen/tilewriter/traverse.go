package tilewriter

import (
	"fmt"
	"sort"
	"sync"

	"FooPkg/tile_gen/geometry"
	"FooPkg/tile_gen/mbtiles"
	"FooPkg/tile_gen/mvt"
	"FooPkg/tile_gen/serial"
)

// TraverseZooms generates all tiles from minZoom to maxZoom and writes them to db.
func TraverseZooms(features []serial.Feature, db *mbtiles.DB, cfg *Config) error {
	// Sort features by spatial index.
	sort.Slice(features, func(i, j int) bool {
		return features[i].Index < features[j].Index
	})

	for z := cfg.MaxZoom; z >= cfg.MinZoom; z-- {
		if err := writeZoom(features, z, db, cfg); err != nil {
			return fmt.Errorf("zoom %d: %w", z, err)
		}
	}
	return nil
}

func writeZoom(allFeatures []serial.Feature, z int, db *mbtiles.DB, cfg *Config) error {
	// Group features by tile.
	type tileKey struct{ x, y int64 }
	tileMap := map[tileKey][]serial.Feature{}

	for _, f := range allFeatures {
		if !f.VisibleAt(z) {
			continue
		}
		// A feature can overlap multiple tiles; use bounding box to find all.
		x0, y0 := tileXY(f.BBox.MinX, f.BBox.MinY, uint(z))
		x1, y1 := tileXY(f.BBox.MaxX, f.BBox.MaxY, uint(z))
		for tx := x0; tx <= x1; tx++ {
			for ty := y0; ty <= y1; ty++ {
				tileMap[tileKey{tx, ty}] = append(tileMap[tileKey{tx, ty}], f)
			}
		}
	}

	type tileResult struct {
		key  tileKey
		data []byte
		err  error
	}

	keys := make([]tileKey, 0, len(tileMap))
	for k := range tileMap {
		keys = append(keys, k)
	}

	results := make([]tileResult, len(keys))
	var wg sync.WaitGroup
	sem := make(chan struct{}, 8) // limit parallelism

	for i, k := range keys {
		wg.Add(1)
		go func(i int, k tileKey, tileFeat []serial.Feature) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			data, err := renderTile(tileFeat, z, k.x, k.y, cfg)
			results[i] = tileResult{k, data, err}
		}(i, k, tileMap[k])
	}
	wg.Wait()

	for _, r := range results {
		if r.err != nil {
			return r.err
		}
		if len(r.data) == 0 {
			continue
		}
		if err := db.WriteTile(z, int(r.key.x), int(r.key.y), r.data); err != nil {
			return fmt.Errorf("write tile %d/%d/%d: %w", z, r.key.x, r.key.y, err)
		}
	}
	return nil
}

func renderTile(features []serial.Feature, z int, tx, ty int64, cfg *Config) ([]byte, error) {
	detail := cfg.FullDetail
	if z < cfg.MaxZoom {
		detail = cfg.LowDetail
	}
	extent := uint32(1 << detail)
	buf := int64(cfg.Buffer)

	// Tile world bounds with buffer.
	shift := uint(32 - uint(z))
	worldX0 := (tx << shift) - buf*(int64(1)<<uint(32-uint(z)-uint(detail)))
	worldY0 := (ty << shift) - buf*(int64(1)<<uint(32-uint(z)-uint(detail)))
	worldX1 := ((tx + 1) << shift) + buf*(int64(1)<<uint(32-uint(z)-uint(detail)))
	worldY1 := ((ty + 1) << shift) + buf*(int64(1)<<uint(32-uint(z)-uint(detail)))

	// Apply dropping strategies.
	features = dropByRate(features, z, cfg)
	if cfg.GammaCorrection > 0 {
		pixelSize := int64(1) << uint(32-uint(z)-uint(detail))
		features = applyGamma(features, cfg.GammaCorrection, pixelSize)
	}

	// Filter to features that actually intersect this tile's buffered area.
	tileFeatures := features[:0:0]
	for _, f := range features {
		bb := f.BBox
		if bb.MaxX < worldX0 || bb.MinX > worldX1 || bb.MaxY < worldY0 || bb.MinY > worldY1 {
			continue
		}
		tileFeatures = append(tileFeatures, f)
	}

	if len(tileFeatures) == 0 {
		return nil, nil
	}

	// Apply per-feature attribute filtering.
	if cfg.ExcludeAll || len(cfg.IncludeKeys) > 0 || len(cfg.ExcludeKeys) > 0 {
		for i := range tileFeatures {
			tileFeatures[i].Props = filterProps(tileFeatures[i].Props, cfg)
		}
	}

	// Process geometry per feature.
	layerName := cfg.LayerName
	if layerName == "" {
		layerName = tileFeatures[0].Layer
	}

	// Group by layer name.
	layerMap := map[string][]serial.Feature{}
	for _, f := range tileFeatures {
		lname := f.Layer
		if cfg.LayerName != "" {
			lname = cfg.LayerName
		}

		// Transform geometry to tile coordinates.
		processed := processFeatureGeometry(f, z, tx, ty, detail, buf, worldX0, worldY0, worldX1, worldY1, cfg)
		if processed == nil {
			continue
		}
		f.Geometry = processed
		layerMap[lname] = append(layerMap[lname], f)
	}

	if len(layerMap) == 0 {
		return nil, nil
	}

	// Check feature count limit.
	if cfg.MaxTileFeatures > 0 {
		total := 0
		for _, fs := range layerMap {
			total += len(fs)
		}
		if total > cfg.MaxTileFeatures {
			// Trim to limit (simplistic: trim last layer).
			for k, fs := range layerMap {
				if total <= cfg.MaxTileFeatures {
					break
				}
				trim := total - cfg.MaxTileFeatures
				if trim >= len(fs) {
					delete(layerMap, k)
					total -= len(fs)
				} else {
					layerMap[k] = fs[:len(fs)-trim]
					total = cfg.MaxTileFeatures
				}
			}
		}
	}

	data, err := mvt.EncodeTile(layerMap, extent)
	if err != nil {
		return nil, err
	}

	// If tile is too large, try re-rendering with reduced detail.
	if cfg.MaxTileBytes > 0 && len(data) > cfg.MaxTileBytes {
		reducedDetail := detail - 1
		if reducedDetail >= cfg.MinDetail {
			cfg2 := *cfg
			cfg2.FullDetail = reducedDetail
			cfg2.LowDetail = reducedDetail
			return renderTile(features, z, tx, ty, &cfg2)
		}
		// Apply dynamic dropping as last resort.
		if cfg.DropDensestAsNeeded {
			for k, fs := range layerMap {
				layerMap[k] = dropDensest(fs, cfg.MaxTileBytes/len(layerMap))
			}
			data, err = mvt.EncodeTile(layerMap, extent)
		}
	}

	_ = layerName
	return data, err
}

func processFeatureGeometry(
	f serial.Feature, z int, tx, ty int64,
	detail int, buf int64,
	worldX0, worldY0, worldX1, worldY1 int64,
	cfg *Config,
) geometry.DrawVec {
	dv := f.Geometry

	// Scale to tile pixel coordinates.
	scaled := geometry.ScaleToTile(dv, uint(z), tx, ty, detail, buf)
	if len(scaled) == 0 {
		return nil
	}

	// Simplify lines and polygons.
	if !cfg.NoLineSimplification {
		shouldSimplify := z < cfg.MaxZoom || !cfg.SimplifyOnlyLowZooms
		if shouldSimplify {
			tol := float64(1) * cfg.Simplification
			switch f.Type {
			case geometry.TypeLineString:
				scaled = geometry.SimplifyLine(scaled, tol)
			case geometry.TypePolygon:
				scaled = geometry.SimplifyPolygon(scaled, tol)
			}
		}
	}

	// Clip to tile bounds (with buffer in pixel space).
	extent := int64(1 << detail)
	switch f.Type {
	case geometry.TypePoint:
		if !geometry.ClipPoint(scaled[0].X, scaled[0].Y, -buf, -buf, extent+buf, extent+buf) {
			return nil
		}
	case geometry.TypeLineString:
		segs := geometry.ClipLine(scaled, -buf, -buf, extent+buf, extent+buf)
		if len(segs) == 0 {
			return nil
		}
		// Flatten segments.
		var flat geometry.DrawVec
		for _, s := range segs {
			flat = append(flat, s...)
		}
		scaled = flat
	case geometry.TypePolygon:
		clipped := geometry.ClipPolygon(scaled, -buf, -buf, extent+buf, extent+buf)
		if len(clipped) == 0 {
			return nil
		}
		scaled = clipped
	}

	if len(scaled) == 0 {
		return nil
	}
	return scaled
}

func tileXY(wx, wy int64, z uint) (int64, int64) {
	shift := uint(32 - z)
	return wx >> shift, wy >> shift
}

func filterProps(props map[string]serial.Value, cfg *Config) map[string]serial.Value {
	if cfg.ExcludeAll {
		return nil
	}
	if len(cfg.IncludeKeys) > 0 {
		include := make(map[string]struct{}, len(cfg.IncludeKeys))
		for _, k := range cfg.IncludeKeys {
			include[k] = struct{}{}
		}
		out := make(map[string]serial.Value, len(cfg.IncludeKeys))
		for k, v := range props {
			if _, ok := include[k]; ok {
				out[k] = v
			}
		}
		return out
	}
	if len(cfg.ExcludeKeys) > 0 {
		exclude := make(map[string]struct{}, len(cfg.ExcludeKeys))
		for _, k := range cfg.ExcludeKeys {
			exclude[k] = struct{}{}
		}
		out := make(map[string]serial.Value, len(props))
		for k, v := range props {
			if _, ok := exclude[k]; !ok {
				out[k] = v
			}
		}
		return out
	}
	return props
}
