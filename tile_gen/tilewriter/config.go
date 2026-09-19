package tilewriter

// IndexType controls the spatial sort order.
type IndexType int

const (
	IndexHilbert  IndexType = iota // Hilbert curve (default; best spatial locality)
	IndexQuadkey                   // Z-order / Morton code
)

// Config holds all generation parameters. Zero value is not valid — use DefaultConfig.
type Config struct {
	// Zoom range.
	MinZoom int
	MaxZoom int

	// Tile detail (resolution = 1 << FullDetail pixels per axis).
	FullDetail int // at MaxZoom (default 12 → 4096)
	LowDetail  int // below MaxZoom (default 12)
	MinDetail  int // fallback if tile exceeds size limit (default 7)

	// Feature dropping.
	DropRate float64 // features kept per zoom step (default 2.5)
	BaseZoom int     // zoom where all features are included (default = MaxZoom)

	// Dynamic dropping strategies.
	DropDensestAsNeeded   bool
	DropFractionAsNeeded  bool
	DropSmallestAsNeeded  bool
	CoalesceDensestAsNeeded bool

	// Simplification.
	Simplification        float64 // multiplier for Douglas-Peucker tolerance (default 1.0)
	NoLineSimplification  bool
	SimplifyOnlyLowZooms  bool

	// Clipping buffer (1/256ths of a tile, default 5).
	Buffer int

	// Tile size constraints.
	MaxTileBytes    int // default 500_000
	MaxTileFeatures int // default 200_000

	// Gamma for dense point thinning (0 = disabled).
	GammaCorrection float64

	// Attribute filtering.
	IncludeKeys []string // if set, only these keys are kept
	ExcludeKeys []string // keys to drop
	ExcludeAll  bool     // drop all attributes

	// Clustering.
	ClusterDensestAsNeeded bool
	ClusterDistance        int // pixels

	// Spatial index strategy.
	IndexType IndexType

	// Metadata.
	Name        string
	Description string
	Attribution string
	LayerName   string // override input filename as layer name
}

// DefaultConfig returns a Config with tippecanoe-equivalent defaults.
func DefaultConfig() Config {
	return Config{
		MinZoom:         0,
		MaxZoom:         14,
		FullDetail:      12,
		LowDetail:       12,
		MinDetail:       7,
		DropRate:        2.5,
		BaseZoom:        -1, // -1 means "use MaxZoom"
		Buffer:          5,
		MaxTileBytes:    500_000,
		MaxTileFeatures: 200_000,
		Simplification:  1.0,
		IndexType:       IndexHilbert,
	}
}

// baseZoom returns the effective BaseZoom.
func (c *Config) baseZoom() int {
	if c.BaseZoom < 0 {
		return c.MaxZoom
	}
	return c.BaseZoom
}

// extent returns the tile pixel extent at zoom z.
func (c *Config) extent(z int) uint32 {
	detail := c.FullDetail
	if z < c.MaxZoom {
		detail = c.LowDetail
	}
	return uint32(1 << detail)
}
