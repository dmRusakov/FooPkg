package projection

// QuadkeyIndex computes the Z-order (Morton) curve index for (x, y).
func QuadkeyIndex(x, y int64) uint64 {
	return interleave(uint64(x), uint64(y))
}

// interleave spreads bits of x into even positions, y into odd positions.
func interleave(x, y uint64) uint64 {
	x = spread(x)
	y = spread(y)
	return x | (y << 1)
}

func spread(v uint64) uint64 {
	v = (v | (v << 16)) & 0x0000FFFF0000FFFF
	v = (v | (v << 8)) & 0x00FF00FF00FF00FF
	v = (v | (v << 4)) & 0x0F0F0F0F0F0F0F0F
	v = (v | (v << 2)) & 0x3333333333333333
	v = (v | (v << 1)) & 0x5555555555555555
	return v
}

// QuadkeyDecode extracts (x, y) from a Morton code.
func QuadkeyDecode(d uint64) (x, y int64) {
	x = int64(compact(d))
	y = int64(compact(d >> 1))
	return
}

func compact(v uint64) uint64 {
	v &= 0x5555555555555555
	v = (v | (v >> 1)) & 0x3333333333333333
	v = (v | (v >> 2)) & 0x0F0F0F0F0F0F0F0F
	v = (v | (v >> 4)) & 0x00FF00FF00FF00FF
	v = (v | (v >> 8)) & 0x0000FFFF0000FFFF
	v = (v | (v >> 16)) & 0x00000000FFFFFFFF
	return v
}
