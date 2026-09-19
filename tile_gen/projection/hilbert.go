package projection

// HilbertIndex converts (x, y) to a Hilbert curve index.
// Both x and y must be in [0, 2^n) where n is the order.
func HilbertIndex(x, y uint64, order uint) uint64 {
	var d uint64
	for s := uint64(1) << (order - 1); s > 0; s >>= 1 {
		var rx, ry uint64
		if x&s > 0 {
			rx = 1
		}
		if y&s > 0 {
			ry = 1
		}
		d += s * s * ((3 * rx) ^ ry)
		x, y = hilbertRotate(s, x, y, rx, ry)
	}
	return d
}

// HilbertDecode converts a Hilbert index back to (x, y).
func HilbertDecode(d uint64, order uint) (x, y uint64) {
	for s := uint64(1); s < 1<<order; s <<= 1 {
		var rx, ry uint64
		if (d & 2) != 0 {
			ry = 1
		}
		if (d^ry)&1 != 0 {
			rx = 1
		}
		x, y = hilbertRotate(s, x, y, rx, ry)
		x += s * rx
		y += s * ry
		d >>= 2
	}
	return
}

func hilbertRotate(n, x, y, rx, ry uint64) (uint64, uint64) {
	if ry == 0 {
		if rx == 1 {
			x = n - 1 - x
			y = n - 1 - y
		}
		x, y = y, x
	}
	return x, y
}

// HilbertIndexXY returns the Hilbert index for world coordinates at a given zoom.
// Uses 32-bit precision (order 32).
func HilbertIndexXY(wx, wy int64) uint64 {
	return HilbertIndex(uint64(wx), uint64(wy), TileCoordBits)
}
