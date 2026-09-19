package geojson

import (
	"encoding/json"
	"fmt"
	"io"

	"FooPkg/tile_gen/geometry"
	"FooPkg/tile_gen/projection"
	"FooPkg/tile_gen/serial"
)

// Feature is an intermediate GeoJSON feature during parsing.
type rawFeature struct {
	Type       string                 `json:"type"`
	ID         interface{}            `json:"id,omitempty"`
	Geometry   rawGeometry            `json:"geometry"`
	Properties map[string]interface{} `json:"properties"`
}

type rawGeometry struct {
	Type        string          `json:"type"`
	Coordinates json.RawMessage `json:"coordinates"`
	Geometries  []rawGeometry   `json:"geometries,omitempty"`
}

type rawFeatureCollection struct {
	Type     string       `json:"type"`
	Features []rawFeature `json:"features"`
}

// ParseReader reads a GeoJSON FeatureCollection from r and returns serial features.
// layerName is used as the layer field in each feature.
func ParseReader(r io.Reader, layerName string) ([]serial.Feature, error) {
	var fc rawFeatureCollection
	if err := json.NewDecoder(r).Decode(&fc); err != nil {
		return nil, fmt.Errorf("geojson decode: %w", err)
	}
	if fc.Type != "FeatureCollection" {
		return nil, fmt.Errorf("expected FeatureCollection, got %q", fc.Type)
	}

	features := make([]serial.Feature, 0, len(fc.Features))
	for i, f := range fc.Features {
		sf, err := convertFeature(f, layerName)
		if err != nil {
			return nil, fmt.Errorf("feature %d: %w", i, err)
		}
		features = append(features, sf...)
	}
	return features, nil
}

// ParseGeometryJSON parses a GeoJSON geometry object string (from ST_AsGeoJSON).
func ParseGeometryJSON(geomJSON string) ([]geometry.DrawVec, geometry.GeomType, error) {
	var g rawGeometry
	if err := json.Unmarshal([]byte(geomJSON), &g); err != nil {
		return nil, geometry.TypeUnknown, fmt.Errorf("geojson geometry decode: %w", err)
	}
	return parseGeometry(g)
}

func convertFeature(f rawFeature, layer string) ([]serial.Feature, error) {
	props := flattenProperties(f.Properties)

	var featureID uint64
	hasID := false
	if f.ID != nil {
		switch v := f.ID.(type) {
		case float64:
			featureID = uint64(v)
			hasID = true
		case string:
			// skip string IDs
		}
	}

	geoms, gt, err := parseGeometry(f.Geometry)
	if err != nil {
		return nil, err
	}

	out := make([]serial.Feature, 0, len(geoms))
	for _, g := range geoms {
		bb := g.Bounds()
		sf := serial.Feature{
			Layer:    layer,
			Type:     gt,
			Geometry: g,
			Props:    props,
			HasID:    hasID,
			ID:       featureID,
			BBox:     bb,
		}
		out = append(out, sf)
	}
	return out, nil
}

func parseGeometry(g rawGeometry) ([]geometry.DrawVec, geometry.GeomType, error) {
	switch g.Type {
	case "Point":
		c, err := decodePosition(g.Coordinates)
		if err != nil {
			return nil, geometry.TypeUnknown, err
		}
		x, y := projection.LonLatToTile(c[0], c[1])
		dv := geometry.DrawVec{{Op: geometry.MoveTo, X: x, Y: y}}
		return []geometry.DrawVec{dv}, geometry.TypePoint, nil

	case "MultiPoint":
		var cs [][]float64
		if err := json.Unmarshal(g.Coordinates, &cs); err != nil {
			return nil, geometry.TypeUnknown, err
		}
		var dvs []geometry.DrawVec
		for _, c := range cs {
			x, y := projection.LonLatToTile(c[0], c[1])
			dvs = append(dvs, geometry.DrawVec{{Op: geometry.MoveTo, X: x, Y: y}})
		}
		return dvs, geometry.TypePoint, nil

	case "LineString":
		var cs [][]float64
		if err := json.Unmarshal(g.Coordinates, &cs); err != nil {
			return nil, geometry.TypeUnknown, err
		}
		dv := lineStringToDV(cs)
		return []geometry.DrawVec{dv}, geometry.TypeLineString, nil

	case "MultiLineString":
		var css [][][]float64
		if err := json.Unmarshal(g.Coordinates, &css); err != nil {
			return nil, geometry.TypeUnknown, err
		}
		var dvs []geometry.DrawVec
		for _, cs := range css {
			dvs = append(dvs, lineStringToDV(cs))
		}
		return dvs, geometry.TypeLineString, nil

	case "Polygon":
		var rings [][][]float64
		if err := json.Unmarshal(g.Coordinates, &rings); err != nil {
			return nil, geometry.TypeUnknown, err
		}
		return []geometry.DrawVec{polygonToDV(rings)}, geometry.TypePolygon, nil

	case "MultiPolygon":
		var polys [][][][]float64
		if err := json.Unmarshal(g.Coordinates, &polys); err != nil {
			return nil, geometry.TypeUnknown, err
		}
		var dvs []geometry.DrawVec
		for _, rings := range polys {
			dvs = append(dvs, polygonToDV(rings))
		}
		return dvs, geometry.TypePolygon, nil

	case "GeometryCollection":
		var dvs []geometry.DrawVec
		var gt geometry.GeomType
		for _, sub := range g.Geometries {
			subDVs, subGT, err := parseGeometry(sub)
			if err != nil {
				return nil, geometry.TypeUnknown, err
			}
			dvs = append(dvs, subDVs...)
			if gt == geometry.TypeUnknown {
				gt = subGT
			}
		}
		return dvs, gt, nil

	default:
		return nil, geometry.TypeUnknown, fmt.Errorf("unsupported geometry type %q", g.Type)
	}
}

func lineStringToDV(cs [][]float64) geometry.DrawVec {
	if len(cs) == 0 {
		return nil
	}
	dv := make(geometry.DrawVec, len(cs))
	x, y := projection.LonLatToTile(cs[0][0], cs[0][1])
	dv[0] = geometry.Draw{Op: geometry.MoveTo, X: x, Y: y}
	for i, c := range cs[1:] {
		x, y = projection.LonLatToTile(c[0], c[1])
		dv[i+1] = geometry.Draw{Op: geometry.LineTo, X: x, Y: y}
	}
	return dv
}

func polygonToDV(rings [][][]float64) geometry.DrawVec {
	var dv geometry.DrawVec
	for _, ring := range rings {
		if len(ring) < 3 {
			continue
		}
		x, y := projection.LonLatToTile(ring[0][0], ring[0][1])
		dv = append(dv, geometry.Draw{Op: geometry.MoveTo, X: x, Y: y})
		for _, c := range ring[1:] {
			x, y = projection.LonLatToTile(c[0], c[1])
			dv = append(dv, geometry.Draw{Op: geometry.LineTo, X: x, Y: y})
		}
		dv = append(dv, geometry.Draw{Op: geometry.ClosePath})
	}
	return dv
}

func decodePosition(raw json.RawMessage) ([]float64, error) {
	var c []float64
	if err := json.Unmarshal(raw, &c); err != nil {
		return nil, err
	}
	if len(c) < 2 {
		return nil, fmt.Errorf("position needs at least 2 coordinates")
	}
	return c, nil
}

func flattenProperties(props map[string]interface{}) map[string]serial.Value {
	out := make(map[string]serial.Value, len(props))
	for k, v := range props {
		out[k] = serial.ToValue(v)
	}
	return out
}
