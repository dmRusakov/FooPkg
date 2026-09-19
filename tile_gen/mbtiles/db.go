package mbtiles

import (
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite"
)

// DB wraps an mbtiles SQLite database.
type DB struct {
	db   *sql.DB
	stmt *sql.Stmt
}

// Open opens (or creates) an mbtiles file at path.
func Open(path string) (*DB, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open mbtiles %s: %w", path, err)
	}

	if _, err := db.Exec(`PRAGMA journal_mode=WAL`); err != nil {
		db.Close()
		return nil, err
	}
	if _, err := db.Exec(`PRAGMA synchronous=NORMAL`); err != nil {
		db.Close()
		return nil, err
	}

	if err := initSchema(db); err != nil {
		db.Close()
		return nil, err
	}

	stmt, err := db.Prepare(`INSERT OR REPLACE INTO tiles
		(zoom_level, tile_column, tile_row, tile_data)
		VALUES (?, ?, ?, ?)`)
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("prepare insert: %w", err)
	}

	return &DB{db: db, stmt: stmt}, nil
}

func initSchema(db *sql.DB) error {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS metadata (name TEXT PRIMARY KEY, value TEXT);
		CREATE TABLE IF NOT EXISTS tiles (
			zoom_level  INTEGER NOT NULL,
			tile_column INTEGER NOT NULL,
			tile_row    INTEGER NOT NULL,
			tile_data   BLOB,
			UNIQUE (zoom_level, tile_column, tile_row)
		);
	`)
	return err
}

// WriteTile writes a compressed tile blob. tile_row is TMS-flipped (Y = (1<<z)-1-y).
func (d *DB) WriteTile(z, x, y int, data []byte) error {
	tmsY := (1 << z) - 1 - y
	_, err := d.stmt.Exec(z, x, tmsY, data)
	return err
}

// SetMetadata sets a metadata key/value pair.
func (d *DB) SetMetadata(key, value string) error {
	_, err := d.db.Exec(`INSERT OR REPLACE INTO metadata (name, value) VALUES (?, ?)`, key, value)
	return err
}

// GetMetadata retrieves a metadata value.
func (d *DB) GetMetadata(key string) (string, error) {
	var v string
	err := d.db.QueryRow(`SELECT value FROM metadata WHERE name = ?`, key).Scan(&v)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return v, err
}

// ReadTile retrieves a tile blob. Returns nil, nil if not found.
func (d *DB) ReadTile(z, x, y int) ([]byte, error) {
	tmsY := (1 << z) - 1 - y
	var data []byte
	err := d.db.QueryRow(`SELECT tile_data FROM tiles
		WHERE zoom_level=? AND tile_column=? AND tile_row=?`,
		z, x, tmsY).Scan(&data)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return data, err
}

// Close closes the database.
func (d *DB) Close() error {
	if d.stmt != nil {
		d.stmt.Close()
	}
	return d.db.Close()
}

// WriteMetadata writes standard mbtiles metadata fields.
func (d *DB) WriteMetadata(m Metadata) error {
	pairs := map[string]string{
		"name":        m.Name,
		"description": m.Description,
		"version":     m.Version,
		"type":        m.Type,
		"format":      "pbf",
	}
	if m.Attribution != "" {
		pairs["attribution"] = m.Attribution
	}
	if m.MinZoom >= 0 {
		pairs["minzoom"] = fmt.Sprintf("%d", m.MinZoom)
	}
	if m.MaxZoom >= 0 {
		pairs["maxzoom"] = fmt.Sprintf("%d", m.MaxZoom)
	}
	if m.Bounds != "" {
		pairs["bounds"] = m.Bounds
	}
	if m.Center != "" {
		pairs["center"] = m.Center
	}
	for k, v := range pairs {
		if err := d.SetMetadata(k, v); err != nil {
			return err
		}
	}
	return nil
}

// Metadata holds standard mbtiles metadata.
type Metadata struct {
	Name        string
	Description string
	Version     string
	Type        string // "overlay" or "baselayer"
	Attribution string
	MinZoom     int
	MaxZoom     int
	Bounds      string // "minlon,minlat,maxlon,maxlat"
	Center      string // "lon,lat,zoom"
}
