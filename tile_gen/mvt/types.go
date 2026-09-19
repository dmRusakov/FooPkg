package mvt

import "FooPkg/tile_gen/geometry"

// Layer holds decoded MVT layer data.
type Layer struct {
	Name     string
	Extent   uint32
	Features []Feature
}

// Feature holds decoded MVT feature data.
type Feature struct {
	ID       uint64
	HasID    bool
	Type     geometry.GeomType
	Tags     []uint32
	Geometry geometry.DrawVec
	// Decoded key/value pairs (populated by Layer.DecodeFeatureTags).
	Props map[string]interface{}
}

// Tile holds a full MVT tile.
type Tile struct {
	Layers []Layer
}
