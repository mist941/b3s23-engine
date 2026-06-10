package life

func StepSerial(cur, next *Grid) error {
	if cur.W != next.W || cur.H != next.H {
		return ErrSizeMismatch
	}
	w, h := cur.W, cur.H
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			n := 0
			for dy := -1; dy <= 1; dy++ {
				for dx := -1; dx <= 1; dx++ {
					if dx == 0 && dy == 0 {
						continue
					}
					nx := (x + dx + w) % w
					ny := (y + dy + h) % h
					if cur.Get(nx, ny) {
						n++
					}
				}
			}
			next.Set(x, y, Next(cur.Get(x, y), n))
		}
	}
	return nil
}

func Step(cur, next *Grid, pool *WorkerPool) error {
	if cur.W != next.W || cur.H != next.H {
		return ErrSizeMismatch
	}
	if pool == nil || pool.n <= 1 {
		for y := 0; y < cur.H; y++ {
			computeRow(cur, next, y)
		}
		return nil
	}
	pool.run(cur, next)
	return nil
}

func getBit(row []uint64, x int) uint64 {
	return (row[x>>6] >> (uint(x) & 63)) & 1
}

func computeBand(cur, next *Grid, startRow, endRow int) {
	for y := startRow; y < endRow; y++ {
		computeRow(cur, next, y)
	}
}

func computeRow(cur, next *Grid, y int) {
	w, h, wpr := cur.W, cur.H, cur.wordsPerRow

	yUp := y - 1
	if yUp < 0 {
		yUp = h - 1
	}
	yDown := y + 1
	if yDown >= h {
		yDown = 0
	}

	up := cur.words[yUp*wpr : yUp*wpr+wpr]
	mid := cur.words[y*wpr : y*wpr+wpr]
	down := cur.words[yDown*wpr : yDown*wpr+wpr]
	nrow := next.words[y*wpr : y*wpr+wpr]
	for i := range nrow {
		nrow[i] = 0
	}

	emit(nrow, 0, count(up, mid, down, w-1, 0, min(1, w-1)), getBit(mid, 0))

	for x := 1; x < w-1; x++ {
		emit(nrow, x, count(up, mid, down, x-1, x, x+1), getBit(mid, x))
	}

	if w >= 2 {
		emit(nrow, w-1, count(up, mid, down, w-2, w-1, 0), getBit(mid, w-1))
	}
}

func count(up, mid, down []uint64, xl, x, xr int) int {
	return int(getBit(up, xl) + getBit(up, x) + getBit(up, xr) +
		getBit(mid, xl) + getBit(mid, xr) +
		getBit(down, xl) + getBit(down, x) + getBit(down, xr))
}

func emit(nrow []uint64, x, neighbours int, midBit uint64) {
	if Next(midBit == 1, neighbours) {
		nrow[x>>6] |= uint64(1) << (uint(x) & 63)
	}
}
