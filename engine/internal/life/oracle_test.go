package life

type naiveOracle struct {
	w, h  int
	cells [][]bool
}

func oracleFromGrid(g *Grid) *naiveOracle {
	o := &naiveOracle{w: g.W, h: g.H, cells: make([][]bool, g.H)}
	for y := 0; y < g.H; y++ {
		o.cells[y] = make([]bool, g.W)
		for x := 0; x < g.W; x++ {
			o.cells[y][x] = g.Get(x, y)
		}
	}
	return o
}

func (o *naiveOracle) step() *naiveOracle {
	n := &naiveOracle{w: o.w, h: o.h, cells: make([][]bool, o.h)}
	for y := 0; y < o.h; y++ {
		n.cells[y] = make([]bool, o.w)
		for x := 0; x < o.w; x++ {
			c := 0
			for dy := -1; dy <= 1; dy++ {
				for dx := -1; dx <= 1; dx++ {
					if dx == 0 && dy == 0 {
						continue
					}
					nx := ((x+dx)%o.w + o.w) % o.w
					ny := ((y+dy)%o.h + o.h) % o.h
					if o.cells[ny][nx] {
						c++
					}
				}
			}
			alive := o.cells[y][x]
			n.cells[y][x] = c == 3 || (alive && c == 2)
		}
	}
	return n
}

func (o *naiveOracle) equalsGrid(g *Grid) bool {
	if g.W != o.w || g.H != o.h {
		return false
	}
	for y := 0; y < o.h; y++ {
		for x := 0; x < o.w; x++ {
			if o.cells[y][x] != g.Get(x, y) {
				return false
			}
		}
	}
	return true
}
