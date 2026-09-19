package mvt

import (
	"bytes"
	"compress/gzip"
	"encoding/binary"
	"math"

	"FooPkg/tile_gen/geometry"
	"FooPkg/tile_gen/serial"
)

// EncodeTile encodes a map of layer name → features into a gzip-compressed MVT blob.
func EncodeTile(layers map[string][]serial.Feature, extent uint32) ([]byte, error) {
	var buf bytes.Buffer
	for name, features := range layers {
		layerBytes, err := encodeLayer(name, features, extent)
		if err != nil {
			return nil, err
		}
		// field 3 (layers) = tag 3, wire type 2 (length-delimited)
		appendVarint(&buf, (3<<3)|2)
		appendVarint(&buf, uint64(len(layerBytes)))
		buf.Write(layerBytes)
	}

	var out bytes.Buffer
	gz := gzip.NewWriter(&out)
	gz.Write(buf.Bytes())
	gz.Close()
	return out.Bytes(), nil
}

func encodeLayer(name string, features []serial.Feature, extent uint32) ([]byte, error) {
	var buf bytes.Buffer

	// field 15: version = 2
	appendVarint(&buf, (15<<3)|0)
	appendVarint(&buf, 2)

	// field 1: name
	appendBytes(&buf, 1, []byte(name))

	// field 5: extent
	appendVarint(&buf, (5<<3)|0)
	appendVarint(&buf, uint64(extent))

	// Build key/value dictionaries.
	keyIndex := map[string]uint32{}
	valIndex := map[valKey]uint32{}
	var keys []string
	var valKeys []valKey

	addKey := func(k string) uint32 {
		if i, ok := keyIndex[k]; ok {
			return i
		}
		i := uint32(len(keys))
		keyIndex[k] = i
		keys = append(keys, k)
		return i
	}
	addVal := func(v serial.Value) uint32 {
		vk := valKey{t: v.Type, s: v.StrVal, f: v.FloatV, i: v.IntVal, u: v.UintVal, b: v.BoolVal}
		if i, ok := valIndex[vk]; ok {
			return i
		}
		i := uint32(len(valKeys))
		valIndex[vk] = i
		valKeys = append(valKeys, vk)
		return i
	}

	type encodedFeature struct {
		f    serial.Feature
		tags []uint32
	}
	enc := make([]encodedFeature, 0, len(features))
	for _, f := range features {
		var tags []uint32
		for k, v := range f.Props {
			if v.Type == serial.ValNull {
				continue
			}
			ki := addKey(k)
			vi := addVal(v)
			tags = append(tags, ki, vi)
		}
		enc = append(enc, encodedFeature{f, tags})
	}

	// Write keys (field 3).
	for _, k := range keys {
		appendBytes(&buf, 3, []byte(k))
	}

	// Write values (field 4).
	for _, vk := range valKeys {
		vb := encodeValueKey(vk)
		appendBytes(&buf, 4, vb)
	}

	// Write features (field 2).
	for _, e := range enc {
		fb := encodeFeature(e.f, e.tags, extent)
		appendBytes(&buf, 2, fb)
	}

	return buf.Bytes(), nil
}

func encodeFeature(f serial.Feature, tags []uint32, extent uint32) []byte {
	var buf bytes.Buffer

	if f.HasID {
		appendVarint(&buf, (1<<3)|0)
		appendVarint(&buf, f.ID)
	}

	if len(tags) > 0 {
		var tagBuf bytes.Buffer
		for _, t := range tags {
			appendVarint(&tagBuf, uint64(t))
		}
		appendBytes(&buf, 2, tagBuf.Bytes())
	}

	appendVarint(&buf, (3<<3)|0)
	appendVarint(&buf, uint64(f.Type))

	geomBytes := encodeGeometry(f.Geometry)
	appendBytes(&buf, 4, geomBytes)

	return buf.Bytes()
}

// encodeGeometry converts DrawVec to MVT geometry encoding.
// MVT uses delta-encoded, zigzag-compressed coordinates.
func encodeGeometry(dv geometry.DrawVec) []byte {
	var buf bytes.Buffer
	var curX, curY int64

	i := 0
	for i < len(dv) {
		d := dv[i]
		switch d.Op {
		case geometry.MoveTo:
			j := i
			for j < len(dv) && dv[j].Op == geometry.MoveTo {
				j++
			}
			count := j - i
			appendVarint(&buf, uint64((count<<3)|1))
			for k := i; k < j; k++ {
				dx := dv[k].X - curX
				dy := dv[k].Y - curY
				appendVarint(&buf, uint64(zigzag(dx)))
				appendVarint(&buf, uint64(zigzag(dy)))
				curX, curY = dv[k].X, dv[k].Y
			}
			i = j

		case geometry.LineTo:
			j := i
			for j < len(dv) && dv[j].Op == geometry.LineTo {
				j++
			}
			count := j - i
			appendVarint(&buf, uint64((count<<3)|2))
			for k := i; k < j; k++ {
				dx := dv[k].X - curX
				dy := dv[k].Y - curY
				appendVarint(&buf, uint64(zigzag(dx)))
				appendVarint(&buf, uint64(zigzag(dy)))
				curX, curY = dv[k].X, dv[k].Y
			}
			i = j

		case geometry.ClosePath:
			appendVarint(&buf, (1<<3)|7)
			i++

		default:
			i++
		}
	}

	return buf.Bytes()
}

func zigzag(n int64) uint32 {
	return uint32((n << 1) ^ (n >> 63))
}

type valKey struct {
	t serial.ValueType
	s string
	f float64
	i int64
	u uint64
	b bool
}

func encodeValueKey(vk valKey) []byte {
	var buf bytes.Buffer
	switch vk.t {
	case serial.ValString:
		appendBytes(&buf, 1, []byte(vk.s))
	case serial.ValFloat:
		appendVarint(&buf, (2<<3)|5)
		b := make([]byte, 4)
		binary.LittleEndian.PutUint32(b, math.Float32bits(float32(vk.f)))
		buf.Write(b)
	case serial.ValDouble:
		appendVarint(&buf, (3<<3)|1)
		b := make([]byte, 8)
		binary.LittleEndian.PutUint64(b, math.Float64bits(vk.f))
		buf.Write(b)
	case serial.ValInt:
		appendVarint(&buf, (4<<3)|0)
		appendVarint(&buf, uint64(vk.i))
	case serial.ValUint:
		appendVarint(&buf, (5<<3)|0)
		appendVarint(&buf, vk.u)
	case serial.ValSint:
		appendVarint(&buf, (6<<3)|0)
		appendSVarint(&buf, vk.i)
	case serial.ValBool:
		appendVarint(&buf, (7<<3)|0)
		if vk.b {
			appendVarint(&buf, 1)
		} else {
			appendVarint(&buf, 0)
		}
	}
	return buf.Bytes()
}

func appendVarint(buf *bytes.Buffer, v uint64) {
	var tmp [binary.MaxVarintLen64]byte
	n := binary.PutUvarint(tmp[:], v)
	buf.Write(tmp[:n])
}

func appendSVarint(buf *bytes.Buffer, v int64) {
	var tmp [binary.MaxVarintLen64]byte
	n := binary.PutVarint(tmp[:], v)
	buf.Write(tmp[:n])
}

func appendBytes(buf *bytes.Buffer, field uint64, data []byte) {
	appendVarint(buf, (field<<3)|2)
	appendVarint(buf, uint64(len(data)))
	buf.Write(data)
}
