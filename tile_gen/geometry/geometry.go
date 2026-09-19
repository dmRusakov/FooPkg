package geometry

// GeomType mirrors MVT geometry types.
type GeomType int

const (
	TypeUnknown    GeomType = 0
	TypePoint      GeomType = 1
	TypeLineString GeomType = 2
	TypePolygon    GeomType = 3
)

// Op is a draw command.
type Op int8

const (
	MoveTo    Op = 1
	LineTo    Op = 2
	ClosePath Op = 7
)

// Draw is a single draw command with coordinates.
type Draw struct {
	X  int64
	Y  int64
	Op Op
	// Necessary marks the point as required (not simplifiable).
	Necessary bool
}

// DrawVec is a sequence of draw commands representing one geometry.
type DrawVec []Draw

// BBox is an axis-aligned bounding box.
type BBox struct {
	MinX, MinY, MaxX, MaxY int64
}

// Bounds computes the bounding box of a DrawVec.
func (dv DrawVec) Bounds() BBox {
	if len(dv) == 0 {
		return BBox{}
	}
	bb := BBox{MinX: dv[0].X, MinY: dv[0].Y, MaxX: dv[0].X, MaxY: dv[0].Y}
	for _, d := range dv[1:] {
		if d.Op == ClosePath {
			continue
		}
		if d.X < bb.MinX {
			bb.MinX = d.X
		}
		if d.X > bb.MaxX {
			bb.MaxX = d.X
		}
		if d.Y < bb.MinY {
			bb.MinY = d.Y
		}
		if d.Y > bb.MaxY {
			bb.MaxY = d.Y
		}
	}
	return bb
}

// Area computes the signed area of a polygon ring (positive = counter-clockwise).
func RingArea(ring DrawVec) int64 {
	var area int64
	j := len(ring) - 1
	for i, d := range ring {
		if d.Op == ClosePath {
			continue
		}
		pj := ring[j]
		area += pj.X * d.Y
		area -= d.X * pj.Y
		j = i
	}
	return area / 2
}

// Rings splits a polygon DrawVec into its component rings.
func Rings(dv DrawVec) []DrawVec {
	var rings []DrawVec
	var cur DrawVec
	for _, d := range dv {
		cur = append(cur, d)
		if d.Op == ClosePath {
			rings = append(rings, cur)
			cur = nil
		}
	}
	if len(cur) > 0 {
		rings = append(rings, cur)
	}
	return rings
}
