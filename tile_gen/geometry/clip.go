package geometry

// ClipPoint returns true if the point is within the clip box (inclusive).
func ClipPoint(x, y, x0, y0, x1, y1 int64) bool {
	return x >= x0 && x <= x1 && y >= y0 && y <= y1
}

// ClipLine clips a LineString DrawVec to the rectangle [x0,y0]×[x1,y1].
// Segments crossing the boundary are split at the intersection.
func ClipLine(dv DrawVec, x0, y0, x1, y1 int64) []DrawVec {
	var segments []DrawVec
	var current DrawVec

	flush := func() {
		if len(current) >= 2 {
			segments = append(segments, current)
		}
		current = nil
	}

	inBounds := func(x, y int64) bool {
		return x >= x0 && x <= x1 && y >= y0 && y <= y1
	}

	var prevX, prevY int64
	hasPrev := false

	for _, d := range dv {
		if d.Op == MoveTo {
			flush()
			prevX, prevY = d.X, d.Y
			hasPrev = true
			if inBounds(d.X, d.Y) {
				current = append(current, d)
			}
			continue
		}
		if d.Op == ClosePath {
			flush()
			hasPrev = false
			continue
		}
		if !hasPrev {
			prevX, prevY = d.X, d.Y
			hasPrev = true
			continue
		}

		x2, y2 := d.X, d.Y
		cx1, cy1, cx2, cy2, ok := cohenSutherlandClip(prevX, prevY, x2, y2, x0, y0, x1, y1)
		if ok {
			if len(current) == 0 || current[len(current)-1].X != cx1 || current[len(current)-1].Y != cy1 {
				if len(current) > 0 {
					flush()
				}
				current = append(current, Draw{Op: MoveTo, X: cx1, Y: cy1})
			}
			current = append(current, Draw{Op: LineTo, X: cx2, Y: cy2})
		} else {
			flush()
		}
		prevX, prevY = x2, y2
	}
	flush()
	return segments
}

// ClipPolygon clips a polygon ring using Sutherland-Hodgman algorithm.
// Returns the clipped ring (without ClosePath; caller must append if needed).
func ClipPolygon(dv DrawVec, x0, y0, x1, y1 int64) DrawVec {
	type pt struct{ x, y int64 }

	// Extract points from ring (skip ClosePath).
	var poly []pt
	for _, d := range dv {
		if d.Op != ClosePath {
			poly = append(poly, pt{d.X, d.Y})
		}
	}
	if len(poly) == 0 {
		return nil
	}

	// Clip against each of the 4 edges.
	clipEdge := func(pts []pt, ex0, ey0, ex1, ey1 int64) []pt {
		if len(pts) == 0 {
			return nil
		}
		inside := func(p pt) bool {
			// Determine "inside" relative to directed edge (ex0,ey0)→(ex1,ey1)
			return (ex1-ex0)*(p.y-ey0)-(ey1-ey0)*(p.x-ex0) >= 0
		}
		intersect := func(a, b pt) pt {
			dx1 := ex1 - ex0
			dy1 := ey1 - ey0
			dx2 := b.x - a.x
			dy2 := b.y - a.y
			denom := dx1*dy2 - dy1*dx2
			if denom == 0 {
				return a
			}
			t := ((a.x-ex0)*dy2 - (a.y-ey0)*dx2)
			xi := ex0 + dx1*t/denom
			yi := ey0 + dy1*t/denom
			return pt{xi, yi}
		}
		var out []pt
		prev := pts[len(pts)-1]
		for _, curr := range pts {
			if inside(curr) {
				if !inside(prev) {
					out = append(out, intersect(prev, curr))
				}
				out = append(out, curr)
			} else if inside(prev) {
				out = append(out, intersect(prev, curr))
			}
			prev = curr
		}
		return out
	}

	poly = clipEdge(poly, x0, y0, x1, y0) // bottom
	poly = clipEdge(poly, x1, y0, x1, y1) // right
	poly = clipEdge(poly, x1, y1, x0, y1) // top
	poly = clipEdge(poly, x0, y1, x0, y0) // left

	if len(poly) < 3 {
		return nil
	}

	out := make(DrawVec, 0, len(poly)+1)
	out = append(out, Draw{Op: MoveTo, X: poly[0].x, Y: poly[0].y})
	for _, p := range poly[1:] {
		out = append(out, Draw{Op: LineTo, X: p.x, Y: p.y})
	}
	out = append(out, Draw{Op: ClosePath})
	return out
}

// cohenSutherlandClip clips segment (x1,y1)-(x2,y2) to rectangle.
// Returns clipped endpoints and whether any part is visible.
func cohenSutherlandClip(x1, y1, x2, y2, minX, minY, maxX, maxY int64) (cx1, cy1, cx2, cy2 int64, ok bool) {
	const (
		inside = 0
		left   = 1
		right  = 2
		bottom = 4
		top    = 8
	)
	code := func(x, y int64) int {
		c := inside
		if x < minX {
			c |= left
		} else if x > maxX {
			c |= right
		}
		if y < minY {
			c |= bottom
		} else if y > maxY {
			c |= top
		}
		return c
	}

	c0, c1 := code(x1, y1), code(x2, y2)
	for {
		if c0|c1 == 0 {
			return x1, y1, x2, y2, true
		}
		if c0&c1 != 0 {
			return 0, 0, 0, 0, false
		}
		cOut := c0
		if cOut == inside {
			cOut = c1
		}
		var x, y int64
		dx := x2 - x1
		dy := y2 - y1
		switch {
		case cOut&top != 0:
			x = x1 + dx*(maxY-y1)/dy
			y = maxY
		case cOut&bottom != 0:
			x = x1 + dx*(minY-y1)/dy
			y = minY
		case cOut&right != 0:
			y = y1 + dy*(maxX-x1)/dx
			x = maxX
		default:
			y = y1 + dy*(minX-x1)/dx
			x = minX
		}
		if cOut == c0 {
			x1, y1 = x, y
			c0 = code(x1, y1)
		} else {
			x2, y2 = x, y
			c1 = code(x2, y2)
		}
	}
}

// ScaleToTile converts world coordinates to tile-local pixel coordinates.
// tileX, tileY are the tile indices at zoom z.
// detail is the number of bits of resolution (e.g. 12 → 4096 pixels).
// buffer is extra pixels outside the tile bounds to allow (prevents edge artifacts).
func ScaleToTile(dv DrawVec, z uint, tileX, tileY int64, detail int, buffer int64) DrawVec {
	shift := uint(32 - z)
	tileOriginX := tileX << shift
	tileOriginY := tileY << shift
	scale := int64(1) << (shift - uint(detail))
	if scale == 0 {
		scale = 1
	}

	out := make(DrawVec, len(dv))
	for i, d := range dv {
		if d.Op == ClosePath {
			out[i] = d
			continue
		}
		out[i] = Draw{
			Op:        d.Op,
			X:         (d.X-tileOriginX)/scale - buffer,
			Y:         (d.Y-tileOriginY)/scale - buffer,
			Necessary: d.Necessary,
		}
	}
	return out
}
