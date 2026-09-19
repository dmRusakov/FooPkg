package geometry

import "math"

// SimplifyLine applies Douglas-Peucker simplification to a LineString DrawVec.
// tolerance is in the same units as the coordinates.
// Points marked Necessary are always kept.
func SimplifyLine(dv DrawVec, tolerance float64) DrawVec {
	if len(dv) <= 2 {
		return dv
	}
	kept := make([]bool, len(dv))
	kept[0] = true
	kept[len(dv)-1] = true
	for i, d := range dv {
		if d.Necessary {
			kept[i] = true
		}
	}
	dpReduce(dv, 0, len(dv)-1, tolerance*tolerance, kept)

	out := dv[:0:0]
	for i, d := range dv {
		if kept[i] {
			out = append(out, d)
		}
	}
	return out
}

// SimplifyPolygon simplifies each ring of a polygon DrawVec.
func SimplifyPolygon(dv DrawVec, tolerance float64) DrawVec {
	rings := Rings(dv)
	var out DrawVec
	for _, ring := range rings {
		simplified := simplifyRing(ring, tolerance)
		out = append(out, simplified...)
	}
	return out
}

func simplifyRing(ring DrawVec, tolerance float64) DrawVec {
	// Polygon rings need at least 4 points (3 + close).
	if len(ring) < 4 {
		return ring
	}
	kept := make([]bool, len(ring))
	kept[0] = true
	kept[len(ring)-2] = true // point before ClosePath
	for i, d := range ring {
		if d.Necessary {
			kept[i] = true
		}
	}
	dpReduce(ring, 0, len(ring)-2, tolerance*tolerance, kept)

	out := ring[:0:0]
	for i, d := range ring {
		if kept[i] || d.Op == ClosePath {
			out = append(out, d)
		}
	}
	if len(out) < 4 {
		return ring
	}
	return out
}

func dpReduce(dv DrawVec, start, end int, sqTol float64, kept []bool) {
	if end-start < 2 {
		return
	}
	maxSq := 0.0
	maxIdx := start + 1
	for i := start + 1; i < end; i++ {
		if kept[i] {
			continue
		}
		sq := sqSegmentDistance(dv[i], dv[start], dv[end])
		if sq > maxSq {
			maxSq = sq
			maxIdx = i
		}
	}
	if maxSq > sqTol {
		kept[maxIdx] = true
		dpReduce(dv, start, maxIdx, sqTol, kept)
		dpReduce(dv, maxIdx, end, sqTol, kept)
	}
}

// sqSegmentDistance returns squared distance from point p to segment (a, b).
func sqSegmentDistance(p, a, b Draw) float64 {
	dx := float64(b.X - a.X)
	dy := float64(b.Y - a.Y)
	if dx == 0 && dy == 0 {
		return sqDist(p, a)
	}
	t := (float64(p.X-a.X)*dx + float64(p.Y-a.Y)*dy) / (dx*dx + dy*dy)
	if t < 0 {
		return sqDist(p, a)
	}
	if t > 1 {
		return sqDist(p, b)
	}
	proj := Draw{X: a.X + int64(t*dx), Y: a.Y + int64(t*dy)}
	return sqDist(p, proj)
}

func sqDist(a, b Draw) float64 {
	dx := float64(a.X - b.X)
	dy := float64(a.Y - b.Y)
	return dx*dx + dy*dy
}

// SimplifyTolerance returns the Douglas-Peucker tolerance for a given zoom level
// and detail bits, scaled by a multiplier.
func SimplifyTolerance(zoom, maxZoom uint, detail int, multiplier float64) float64 {
	// One pixel at this zoom in world coordinates:
	pixelSize := float64(int64(1) << (32 - uint(detail) - zoom))
	return pixelSize * multiplier * math.Sqrt2
}
