package tilewriter

import (
	"math"
	"sort"

	"FooPkg/tile_gen/geometry"
	"FooPkg/tile_gen/serial"
)

// dropByRate drops features at zooms below baseZoom using the configured drop rate.
// Each step down from baseZoom, features are kept with probability 1/dropRate.
// Uses the feature's spatial index as a consistent hash for deterministic dropping.
func dropByRate(features []serial.Feature, z int, cfg *Config) []serial.Feature {
	base := cfg.baseZoom()
	if z >= base {
		return features
	}

	// Fraction of features to keep at this zoom.
	fraction := math.Pow(1.0/cfg.DropRate, float64(base-z))
	threshold := uint64(fraction * math.MaxUint64)

	out := features[:0:0]
	for _, f := range features {
		if f.Type != geometry.TypePoint {
			out = append(out, f)
			continue
		}
		if f.Index <= threshold {
			out = append(out, f)
		}
	}
	return out
}

// dropDensest dynamically drops the densest-packed features until the estimated
// tile size is within maxBytes.
func dropDensest(features []serial.Feature, maxBytes int) []serial.Feature {
	if len(features) == 0 {
		return features
	}

	// Sort by spatial index so nearby features are adjacent.
	sorted := make([]serial.Feature, len(features))
	copy(sorted, features)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Index < sorted[j].Index
	})

	// Binary search for the minimum spacing that keeps estimated size under limit.
	lo, hi := int64(0), int64(1)<<32

	estimate := func(minSpacing int64) int {
		var kept []serial.Feature
		var lastX, lastY int64
		for _, f := range sorted {
			if f.Type != geometry.TypePoint {
				kept = append(kept, f)
				continue
			}
			cx := (f.BBox.MinX + f.BBox.MaxX) / 2
			cy := (f.BBox.MinY + f.BBox.MaxY) / 2
			dx := cx - lastX
			dy := cy - lastY
			if dx*dx+dy*dy >= minSpacing*minSpacing {
				kept = append(kept, f)
				lastX, lastY = cx, cy
			}
		}
		return estimateTileSize(kept)
	}

	if estimate(lo) <= maxBytes {
		return sorted
	}

	for lo < hi-1 {
		mid := (lo + hi) / 2
		if estimate(mid) <= maxBytes {
			hi = mid
		} else {
			lo = mid
		}
	}

	// Apply hi spacing.
	var out []serial.Feature
	var lastX, lastY int64
	for _, f := range sorted {
		if f.Type != geometry.TypePoint {
			out = append(out, f)
			continue
		}
		cx := (f.BBox.MinX + f.BBox.MaxX) / 2
		cy := (f.BBox.MinY + f.BBox.MaxY) / 2
		dx := cx - lastX
		dy := cy - lastY
		if dx*dx+dy*dy >= hi*hi {
			out = append(out, f)
			lastX, lastY = cx, cy
		}
	}
	return out
}

// dropSmallest drops the smallest polygon/line features first to reduce tile size.
func dropSmallest(features []serial.Feature, maxBytes int) []serial.Feature {
	if estimateTileSize(features) <= maxBytes {
		return features
	}

	type featureArea struct {
		f    serial.Feature
		area int64
	}
	areas := make([]featureArea, len(features))
	for i, f := range features {
		areas[i] = featureArea{f, featureExtent(f)}
	}
	sort.Slice(areas, func(i, j int) bool {
		return areas[i].area > areas[j].area // largest first
	})

	var out []serial.Feature
	for _, a := range areas {
		out = append(out, a.f)
		if estimateTileSize(out) <= maxBytes {
			break
		}
	}
	return out
}

// dropFraction drops a fixed fraction of all features across a zoom level.
func dropFraction(features []serial.Feature, maxBytes int) []serial.Feature {
	if estimateTileSize(features) <= maxBytes {
		return features
	}
	// Binary search on fraction to keep.
	lo, hi := 0.0, 1.0
	for hi-lo > 0.01 {
		mid := (lo + hi) / 2
		n := int(float64(len(features)) * mid)
		if estimateTileSize(features[:n]) <= maxBytes {
			lo = mid
		} else {
			hi = mid
		}
	}
	n := int(float64(len(features)) * lo)
	return features[:n]
}

// applyGamma thins point clusters closer than 1 pixel using gamma correction.
// Gamma = 2 keeps sqrt(N) points from a cluster of N; gamma = 1 keeps all.
func applyGamma(features []serial.Feature, gamma float64, pixelSize int64) []serial.Feature {
	if gamma <= 0 || gamma == 1 {
		return features
	}
	out := make([]serial.Feature, 0, len(features))
	for i, f := range features {
		if f.Type != geometry.TypePoint {
			out = append(out, f)
			continue
		}
		// Count nearby points.
		cx := (f.BBox.MinX + f.BBox.MaxX) / 2
		cy := (f.BBox.MinY + f.BBox.MaxY) / 2
		clusterCount := 0
		for _, g := range features {
			if g.Type != geometry.TypePoint {
				continue
			}
			gx := (g.BBox.MinX + g.BBox.MaxX) / 2
			gy := (g.BBox.MinY + g.BBox.MaxY) / 2
			dx := cx - gx
			dy := cy - gy
			if dx*dx+dy*dy < pixelSize*pixelSize {
				clusterCount++
			}
		}
		if clusterCount <= 1 {
			out = append(out, f)
			continue
		}
		// Keep with probability N^(1/gamma - 1).
		prob := math.Pow(float64(clusterCount), 1.0/gamma-1.0)
		threshold := uint64(prob * float64(math.MaxUint64))
		if features[i].Index <= threshold {
			out = append(out, f)
		}
	}
	return out
}

// estimateTileSize returns a rough byte estimate for a feature set.
// Used for dynamic dropping decisions.
func estimateTileSize(features []serial.Feature) int {
	total := 0
	for _, f := range features {
		total += 2 * len(f.Geometry) // coords
		for k, v := range f.Props {
			total += len(k) + len(v.StrVal) + 8
		}
	}
	return total
}

// featureExtent returns the bounding-box area for sort purposes.
func featureExtent(f serial.Feature) int64 {
	bb := f.BBox
	return (bb.MaxX - bb.MinX) * (bb.MaxY - bb.MinY)
}
