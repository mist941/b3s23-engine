package life

import "math/rand/v2"

func Seed(g *Grid, prob float64, rngSeed uint64) {
	g.Clear()
	switch {
	case prob <= 0:
		return
	case prob >= 1:
		for i := range g.words {
			g.words[i] = ^uint64(0)
		}
		g.maskTrailingBits()
		return
	}

	r := rand.New(rand.NewPCG(rngSeed, rngSeed^0x9e3779b97f4a7c15))
	for y := 0; y < g.H; y++ {
		for x := 0; x < g.W; x++ {
			if r.Float64() < prob {
				g.Set(x, y, true)
			}
		}
	}
}
