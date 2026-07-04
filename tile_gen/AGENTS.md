# tile_gen — Vector Tile Generation Package

Go reimplementation of [tippecanoe](https://github.com/felt/tippecanoe) core functionality.
Converts GeoJSON input into Mapbox Vector Tiles (MVT) stored as mbtiles (SQLite) or a directory tree.

## Package Layout

```
tile_gen/
├── geometry/       Coordinate math, Douglas-Peucker simplification, polygon clipping
├── geojson/        GeoJSON parsing → internal feature representation
├── projection/     Lon/lat ↔ tile coordinates; Hilbert curve; quadkey Z-order
├── serial/         Feature serialization (geometry + metadata streams)
├── mvt/            MVT (Mapbox Vector Tiles) Protocol Buffer encoding/decoding
├── mbtiles/        SQLite mbtiles read/write (tiles + metadata tables)
└── tilewriter/     Main pipeline: zoom traversal, feature dropping, tile assembly
```

## Architecture

```
GeoJSON input
    │
    ▼
geojson.Parse()          → []serial.Feature  (world coordinates, int64 precision)
    │
    ▼
projection.LonLatToTile()  (EPSG:4326 → 32-bit tile space)
projection.HilbertIndex()  spatial sort key
    │
    ▼
serial.Serialize()         write geometry + metadata to temp streams
    │
    ▼
tilewriter.TraverseZooms()
    ├─ feature dropping / clustering per zoom
    ├─ geometry.Simplify()       Douglas-Peucker per tile
    ├─ geometry.Clip()           Sutherland-Hodgman to tile bounds + buffer
    └─ mvt.EncodeTile()          protobuf → gzip → []byte
    │
    ▼
mbtiles.Write()            INSERT INTO tiles (zoom_level, tile_column, tile_row, tile_data)
```

## Key Concepts

### Coordinate System
- All internal coordinates are **int64 in 32-bit tile space** (range `[0, 2^32)`).
- `projection.LonLatToTile(lon, lat)` converts WGS84 to this space.
- At zoom Z, a tile covers `2^(32-Z)` units per axis; tile `(x,y)` starts at `(x * 2^(32-Z), y * 2^(32-Z))`.
- Tile detail (resolution) defaults to 12 bits → 4096 × 4096 pixels per tile.

### Spatial Indexing
Two index strategies (set via `Config.IndexType`):
- `IndexHilbert` (default) — Hilbert curve; best spatial locality.
- `IndexQuadkey` — Z-order / Morton code; simpler, slightly worse locality.

### Geometry Types
`geometry.GeomType`: `TypePoint`, `TypeLineString`, `TypePolygon`.
Draw commands: `MoveTo`, `LineTo`, `ClosePath` (mirrors MVT command encoding).

### Feature Dropping
At zooms below `Config.BaseZoom`, a fraction of point features is dropped to control density:
- `Config.DropRate` (default 2.5): ratio of features kept per zoom step up.
- `Config.DropDensestAsNeeded`: dynamic dropping to stay under tile size limit.
- `Config.GammaCorrection`: thin clusters closer than 1 pixel (0 = disabled).

### MVT Encoding
`mvt.EncodeTile(layers)` emits a gzip-compressed Protocol Buffer blob:
- Coordinates are delta-encoded and zigzag-encoded.
- Key/value strings are dictionary-compressed per layer.
- `extent` matches `1 << Config.FullDetail` (default 4096).

### mbtiles Schema
```sql
CREATE TABLE metadata (name TEXT, value TEXT);
CREATE TABLE tiles   (zoom_level INT, tile_column INT, tile_row INT, tile_data BLOB);
```
`tile_row` is stored as `(1 << zoom) - 1 - y` (TMS flip).

## Configuration

`tilewriter.Config` mirrors tippecanoe's major flags:

| Field | Default | tippecanoe flag |
|---|---|---|
| `MaxZoom` | 14 | `-z` |
| `MinZoom` | 0 | `-Z` |
| `FullDetail` | 12 | `-d` |
| `LowDetail` | 12 | `-D` |
| `MinDetail` | 7 | `-m` |
| `DropRate` | 2.5 | `-r` |
| `BaseZoom` | `MaxZoom` | `-B` |
| `Buffer` | 5 | `-b` |
| `Simplification` | 1.0 | `-S` |
| `MaxTileBytes` | 500_000 | `-M` |
| `MaxTileFeatures` | 200_000 | `-O` |
| `GammaCorrection` | 0 | `-g` |
| `IndexType` | `IndexHilbert` | (internal) |

## Adding a New Feature-Drop Strategy

1. Add a bool field to `tilewriter.Config` (e.g., `DropSmallestAsNeeded`).
2. Implement the strategy in `tilewriter/drop.go` — receives a `[]serial.Feature` slice and returns the filtered slice.
3. Call it from `tilewriter/traverse.go` in `writeZoom()` before `mvt.EncodeTile`.

## Testing

Each sub-package has unit tests. Run all:
```
go test FooPkg/tile_gen/...
```
End-to-end smoke test (requires a GeoJSON file):
```go
cfg := tilewriter.DefaultConfig()
cfg.MaxZoom = 5
err := tilewriter.Generate(ctx, "input.geojson", "output.mbtiles", cfg)
```

## External Dependencies

| Package | Purpose |
|---|---|
| `modernc.org/sqlite` | Pure-Go SQLite driver (no CGO) for mbtiles |
| `compress/gzip` | Tile compression (stdlib) |
| `encoding/binary` | Varint encoding for protobuf (stdlib) |
| `encoding/json` | GeoJSON parsing (stdlib) |

## Differences from tippecanoe

- No Geobuf or CSV input (GeoJSON only).
- No `tile-join` utility.
- No Mapbox GL filter expression evaluation (`-j`).
- No parallel I/O segments (single-threaded geometry pass; goroutine-parallel tile writing).
- Directory output (`-e`) not yet implemented; mbtiles only.
