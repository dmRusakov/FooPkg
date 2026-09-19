package serial

import (
	"fmt"
	"math"

	"FooPkg/tile_gen/geometry"
)

// ValueType mirrors MVT attribute types.
type ValueType int

const (
	ValString ValueType = iota
	ValFloat
	ValDouble
	ValInt
	ValUint
	ValSint
	ValBool
	ValNull
)

// Value holds a typed attribute value.
type Value struct {
	Type    ValueType
	StrVal  string
	FloatV  float64
	IntVal  int64
	UintVal uint64
	BoolVal bool
}

// ToValue converts an interface{} (from JSON) to a typed Value.
func ToValue(v interface{}) Value {
	if v == nil {
		return Value{Type: ValNull}
	}
	switch t := v.(type) {
	case bool:
		return Value{Type: ValBool, BoolVal: t}
	case float64:
		if t == math.Trunc(t) && t >= math.MinInt64 && t <= math.MaxInt64 {
			return Value{Type: ValInt, IntVal: int64(t)}
		}
		return Value{Type: ValDouble, FloatV: t}
	case string:
		return Value{Type: ValString, StrVal: t}
	case int:
		return Value{Type: ValInt, IntVal: int64(t)}
	case int64:
		return Value{Type: ValInt, IntVal: t}
	case uint64:
		return Value{Type: ValUint, UintVal: t}
	default:
		return Value{Type: ValString, StrVal: fmt.Sprintf("%v", v)}
	}
}

// Feature is the internal representation of a geographic feature ready for tiling.
type Feature struct {
	Layer    string
	Type     geometry.GeomType
	Geometry geometry.DrawVec
	Props    map[string]Value

	HasID bool
	ID    uint64

	// Spatial index key (Hilbert or quadkey).
	Index uint64

	// Bounding box in 32-bit world coordinates.
	BBox geometry.BBox

	// Per-feature zoom constraints (tippecanoe:minzoom / tippecanoe:maxzoom).
	HasMinZoom bool
	MinZoom    int
	HasMaxZoom bool
	MaxZoom    int

	// Sequence number (insertion order).
	Seq int64
}

// ApplyZoomProperties reads tippecanoe:minzoom and tippecanoe:maxzoom from Props
// into the structured fields, then removes them from Props.
func (f *Feature) ApplyZoomProperties() {
	if v, ok := f.Props["tippecanoe:minzoom"]; ok {
		if v.Type == ValInt {
			f.HasMinZoom = true
			f.MinZoom = int(v.IntVal)
		}
		delete(f.Props, "tippecanoe:minzoom")
	}
	if v, ok := f.Props["tippecanoe:maxzoom"]; ok {
		if v.Type == ValInt {
			f.HasMaxZoom = true
			f.MaxZoom = int(v.IntVal)
		}
		delete(f.Props, "tippecanoe:maxzoom")
	}
}

// VisibleAt returns true if this feature should appear at zoom z.
func (f *Feature) VisibleAt(z int) bool {
	if f.HasMinZoom && z < f.MinZoom {
		return false
	}
	if f.HasMaxZoom && z > f.MaxZoom {
		return false
	}
	return true
}
