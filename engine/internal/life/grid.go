package life

import "math/bits"

type Grid struct {
	W, H        int
	wordsPerRow int
	words       []uint64
}

func NewGrid(w, h int) *Grid {
	if w <= 0 || h <= 0 {
		panic("life: grid dimensions must be positive")
	}
	wpr := (w + 63) / 64
	return &Grid{W: w, H: h, wordsPerRow: wpr, words: make([]uint64, wpr*h)}
}

func NewGridFromWords(w, h int, words []uint64) (*Grid, error) {
	if w <= 0 || h <= 0 {
		return nil, ErrBadDimensions
	}
	wpr := (w + 63) / 64
	if len(words) != wpr*h {
		return nil, ErrBadWordCount
	}
	g := &Grid{W: w, H: h, wordsPerRow: wpr, words: append([]uint64(nil), words...)}
	g.maskTrailingBits()
	return g, nil
}

func (g *Grid) WordsPerRow() int { return g.wordsPerRow }

func (g *Grid) Words() []uint64 { return g.words }

func (g *Grid) trailingOffset() int { return g.W % 64 }

func (g *Grid) Get(x, y int) bool {
	return (g.words[y*g.wordsPerRow+(x>>6)]>>(uint(x)&63))&1 == 1
}

func (g *Grid) Set(x, y int, alive bool) {
	idx := y*g.wordsPerRow + (x >> 6)
	bit := uint64(1) << (uint(x) & 63)
	if alive {
		g.words[idx] |= bit
	} else {
		g.words[idx] &^= bit
	}
}

func (g *Grid) Clear() {
	clear(g.words)
}

func (g *Grid) Clone() *Grid {
	return &Grid{
		W:           g.W,
		H:           g.H,
		wordsPerRow: g.wordsPerRow,
		words:       append([]uint64(nil), g.words...),
	}
}

func (g *Grid) Equal(o *Grid) bool {
	if g.W != o.W || g.H != o.H {
		return false
	}
	for i := range g.words {
		if g.words[i] != o.words[i] {
			return false
		}
	}
	return true
}

func (g *Grid) Population() int {
	n := 0
	for _, w := range g.words {
		n += bits.OnesCount64(w)
	}
	return n
}

func (g *Grid) maskTrailingBits() {
	off := g.trailingOffset()
	if off == 0 {
		return
	}
	mask := (uint64(1) << uint(off)) - 1
	for r := 0; r < g.H; r++ {
		idx := r*g.wordsPerRow + g.wordsPerRow - 1
		g.words[idx] &= mask
	}
}
