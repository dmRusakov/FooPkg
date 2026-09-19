package projection

import (
	"math"
)

const (
	TileCoordBits = 32
	TileCoordMax  = 1 << TileCoordBits
)

// LonLatToTile converts WGS84 lon/lat to 32-bit tile-space coordinates.
// The entire world maps to [0, 2^32) on both axes.
func LonLatToTile(lon, lat float64) (x, y int64) {
	// Clamp latitude to avoid infinity at poles
	if lat > 89.9 {
		lat = 89.9
	}
	if lat < -89.9 {
		lat = -89.9
	}

	latRad := lat * math.Pi / 180.0
	n := math.Pow(2, TileCoordBits)

	fx := (lon + 180.0) / 360.0 * n
	fy := (1.0 - math.Log(math.Tan(latRad)+1.0/math.Cos(latRad))/math.Pi) / 2.0 * n

	x = int64(fx)
	y = int64(fy)

	if x < 0 {
		x = 0
	}
	if x >= TileCoordMax {
		x = TileCoordMax - 1
	}
	if y < 0 {
		y = 0
	}
	if y >= TileCoordMax {
		y = TileCoordMax - 1
	}
	return
}

// TileToLonLat converts 32-bit tile-space coordinates back to WGS84.
func TileToLonLat(x, y int64) (lon, lat float64) {
	n := float64(TileCoordMax)
	lon = float64(x)/n*360.0 - 180.0
	latRad := math.Atan(math.Sinh(math.Pi * (1 - 2*float64(y)/n)))
	lat = latRad * 180.0 / math.Pi
	return
}

// Mercator3857ToTile converts EPSG:3857 meters to 32-bit tile space.
func Mercator3857ToTile(mx, my float64) (x, y int64) {
	const earthCircumference = 20037508.342789244
	lon := mx / earthCircumference * 180.0
	latRad := math.Atan(math.Exp(my/earthCircumference*math.Pi)) * 2 - math.Pi/2
	lat := latRad * 180.0 / math.Pi
	return LonLatToTile(lon, lat)
}

// TileXY returns the tile XY coordinates at zoom z that contain the world point wx, wy.
func TileXY(wx, wy int64, z uint) (tx, ty int64) {
	shift := TileCoordBits - z
	tx = wx >> shift
	ty = wy >> shift
	return
}

// TileBounds returns the world-coordinate bounds of tile (z, tx, ty).
func TileBounds(z uint, tx, ty int64) (x0, y0, x1, y1 int64) {
	shift := uint(TileCoordBits - z)
	x0 = tx << shift
	y0 = ty << shift
	x1 = (tx + 1) << shift
	y1 = (ty + 1) << shift
	return
}
