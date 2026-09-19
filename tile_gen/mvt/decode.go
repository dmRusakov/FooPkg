package mvt

import (
	"bytes"
	"compress/gzip"
	"encoding/binary"
	"fmt"
	"io"
	"math"

	"FooPkg/tile_gen/geometry"
)

// DecodeTile decodes a gzip-compressed MVT blob.
func DecodeTile(data []byte) (*Tile, error) {
	r, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		// Try uncompressed.
		return decodeTileBytes(data)
	}
	raw, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("gzip decompress: %w", err)
	}
	return decodeTileBytes(raw)
}

func decodeTileBytes(data []byte) (*Tile, error) {
	t := &Tile{}
	i := 0
	for i < len(data) {
		tag, n := binary.Uvarint(data[i:])
		if n <= 0 {
			return nil, fmt.Errorf("bad varint at %d", i)
		}
		i += n
		field := tag >> 3
		wireType := tag & 0x7

		switch field {
		case 3: // layers
			if wireType != 2 {
				return nil, fmt.Errorf("expected wire type 2 for layers")
			}
			length, n2 := binary.Uvarint(data[i:])
			if n2 <= 0 {
				return nil, fmt.Errorf("bad layer length varint")
			}
			i += n2
			layerData := data[i : i+int(length)]
			i += int(length)
			layer, err := decodeLayer(layerData)
			if err != nil {
				return nil, err
			}
			t.Layers = append(t.Layers, *layer)
		default:
			i, _ = skipField(data, i, wireType)
		}
	}
	return t, nil
}

func decodeLayer(data []byte) (*Layer, error) {
	l := &Layer{Extent: 4096}
	var keys []string
	var vals []interface{}
	var rawFeatures [][]byte

	i := 0
	for i < len(data) {
		tag, n := binary.Uvarint(data[i:])
		if n <= 0 {
			break
		}
		i += n
		field := tag >> 3
		wireType := tag & 0x7

		switch field {
		case 1: // name
			s, ni, err := readString(data, i)
			if err != nil {
				return nil, err
			}
			l.Name = s
			i = ni
		case 2: // features
			b, ni, err := readBytes(data, i)
			if err != nil {
				return nil, err
			}
			rawFeatures = append(rawFeatures, b)
			i = ni
		case 3: // keys
			s, ni, err := readString(data, i)
			if err != nil {
				return nil, err
			}
			keys = append(keys, s)
			i = ni
		case 4: // values
			b, ni, err := readBytes(data, i)
			if err != nil {
				return nil, err
			}
			v, err := decodeValue(b)
			if err != nil {
				return nil, err
			}
			vals = append(vals, v)
			i = ni
		case 5: // extent
			v, ni := binary.Uvarint(data[i:])
			l.Extent = uint32(v)
			i += ni
		case 15: // version
			_, ni := binary.Uvarint(data[i:])
			i += ni
		default:
			i, _ = skipField(data, i, wireType)
		}
	}

	for _, fb := range rawFeatures {
		f, err := decodeFeature(fb, keys, vals)
		if err != nil {
			return nil, err
		}
		l.Features = append(l.Features, *f)
	}
	return l, nil
}

func decodeFeature(data []byte, keys []string, vals []interface{}) (*Feature, error) {
	f := &Feature{}
	i := 0
	for i < len(data) {
		tag, n := binary.Uvarint(data[i:])
		if n <= 0 {
			break
		}
		i += n
		field := tag >> 3
		wireType := tag & 0x7

		switch field {
		case 1: // id
			v, ni := binary.Uvarint(data[i:])
			f.ID = v
			f.HasID = true
			i += ni
		case 2: // tags
			b, ni, err := readBytes(data, i)
			if err != nil {
				return nil, err
			}
			i = ni
			ti := 0
			for ti < len(b) {
				ki, kn := binary.Uvarint(b[ti:])
				ti += kn
				vi, vn := binary.Uvarint(b[ti:])
				ti += vn
				f.Tags = append(f.Tags, uint32(ki), uint32(vi))
			}
		case 3: // type
			v, ni := binary.Uvarint(data[i:])
			f.Type = geometry.GeomType(v)
			i += ni
		case 4: // geometry
			b, ni, err := readBytes(data, i)
			if err != nil {
				return nil, err
			}
			dv, err := decodeGeometry(b)
			if err != nil {
				return nil, err
			}
			f.Geometry = dv
			i = ni
		default:
			i, _ = skipField(data, i, wireType)
		}
	}

	// Decode tags to Props.
	if len(f.Tags) > 0 {
		f.Props = make(map[string]interface{}, len(f.Tags)/2)
		for j := 0; j+1 < len(f.Tags); j += 2 {
			ki, vi := int(f.Tags[j]), int(f.Tags[j+1])
			if ki < len(keys) && vi < len(vals) {
				f.Props[keys[ki]] = vals[vi]
			}
		}
	}

	return f, nil
}

func decodeGeometry(data []byte) (geometry.DrawVec, error) {
	var dv geometry.DrawVec
	var curX, curY int64
	i := 0
	for i < len(data) {
		cmd, n := binary.Uvarint(data[i:])
		if n <= 0 {
			break
		}
		i += n
		cmdID := cmd & 0x7
		count := cmd >> 3

		switch cmdID {
		case 1: // MoveTo
			for k := uint64(0); k < count; k++ {
				dx, n1 := binary.Uvarint(data[i:])
				i += n1
				dy, n2 := binary.Uvarint(data[i:])
				i += n2
				curX += unzigzag(uint32(dx))
				curY += unzigzag(uint32(dy))
				dv = append(dv, geometry.Draw{Op: geometry.MoveTo, X: curX, Y: curY})
			}
		case 2: // LineTo
			for k := uint64(0); k < count; k++ {
				dx, n1 := binary.Uvarint(data[i:])
				i += n1
				dy, n2 := binary.Uvarint(data[i:])
				i += n2
				curX += unzigzag(uint32(dx))
				curY += unzigzag(uint32(dy))
				dv = append(dv, geometry.Draw{Op: geometry.LineTo, X: curX, Y: curY})
			}
		case 7: // ClosePath
			dv = append(dv, geometry.Draw{Op: geometry.ClosePath})
		}
	}
	return dv, nil
}

func unzigzag(n uint32) int64 {
	return int64((n >> 1) ^ -(n & 1))
}

func decodeValue(data []byte) (interface{}, error) {
	i := 0
	for i < len(data) {
		tag, n := binary.Uvarint(data[i:])
		if n <= 0 {
			break
		}
		i += n
		field := tag >> 3
		wireType := tag & 0x7
		switch field {
		case 1: // string
			s, _, err := readString(data, i)
			if err != nil {
				return nil, err
			}
			return s, nil
		case 2: // float
			if wireType == 5 && i+4 <= len(data) {
				bits := binary.LittleEndian.Uint32(data[i : i+4])
				return math.Float32frombits(bits), nil
			}
		case 3: // double
			if wireType == 1 && i+8 <= len(data) {
				bits := binary.LittleEndian.Uint64(data[i : i+8])
				return math.Float64frombits(bits), nil
			}
		case 4: // int
			v, ni := binary.Uvarint(data[i:])
			i += ni
			return int64(v), nil
		case 5: // uint
			v, ni := binary.Uvarint(data[i:])
			i += ni
			return v, nil
		case 6: // sint
			v, ni := binary.Varint(data[i:])
			i += ni
			return v, nil
		case 7: // bool
			v, ni := binary.Uvarint(data[i:])
			i += ni
			return v != 0, nil
		default:
			i, _ = skipField(data, i, wireType)
		}
	}
	return nil, nil
}

func readString(data []byte, i int) (string, int, error) {
	l, n := binary.Uvarint(data[i:])
	if n <= 0 {
		return "", i, fmt.Errorf("bad string length")
	}
	i += n
	if i+int(l) > len(data) {
		return "", i, fmt.Errorf("string extends past buffer")
	}
	s := string(data[i : i+int(l)])
	return s, i + int(l), nil
}

func readBytes(data []byte, i int) ([]byte, int, error) {
	l, n := binary.Uvarint(data[i:])
	if n <= 0 {
		return nil, i, fmt.Errorf("bad bytes length")
	}
	i += n
	if i+int(l) > len(data) {
		return nil, i, fmt.Errorf("bytes extend past buffer")
	}
	return data[i : i+int(l)], i + int(l), nil
}

func skipField(data []byte, i int, wireType uint64) (int, error) {
	switch wireType {
	case 0: // varint
		for i < len(data) {
			b := data[i]
			i++
			if b&0x80 == 0 {
				break
			}
		}
	case 1: // 64-bit
		i += 8
	case 2: // length-delimited
		l, n := binary.Uvarint(data[i:])
		i += n + int(l)
	case 5: // 32-bit
		i += 4
	}
	return i, nil
}
