package life

import (
	"math/rand/v2"
	"sort"
	"testing"
)

func setCells(g *Grid, cells [][2]int) {
	for _, c := range cells {
		g.Set(c[0], c[1], true)
	}
}

func liveCells(g *Grid) [][2]int {
	var out [][2]int
	for y := 0; y < g.H; y++ {
		for x := 0; x < g.W; x++ {
			if g.Get(x, y) {
				out = append(out, [2]int{x, y})
			}
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i][1] != out[j][1] {
			return out[i][1] < out[j][1]
		}
		return out[i][0] < out[j][0]
	})
	return out
}

func sameCells(a, b [][2]int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func stepN(t *testing.T, g *Grid, pool *WorkerPool, n int) *Grid {
	t.Helper()
	cur := g.Clone()
	for i := 0; i < n; i++ {
		next := NewGrid(g.W, g.H)
		if err := Step(cur, next, pool); err != nil {
			t.Fatalf("Step: %v", err)
		}
		cur = next
	}
	return cur
}

func TestBlockStillLife(t *testing.T) {
	g := NewGrid(8, 8)
	block := [][2]int{{2, 2}, {3, 2}, {2, 3}, {3, 3}}
	setCells(g, block)
	got := stepN(t, g, nil, 1)
	if !sameCells(liveCells(got), block) {
		t.Fatalf("block should be stable, got %v", liveCells(got))
	}
}

func TestBlinkerPeriod2(t *testing.T) {
	g := NewGrid(8, 8)
	horizontal := [][2]int{{2, 3}, {3, 3}, {4, 3}}
	vertical := [][2]int{{3, 2}, {3, 3}, {3, 4}}
	setCells(g, horizontal)

	gen1 := stepN(t, g, nil, 1)
	if !sameCells(liveCells(gen1), vertical) {
		t.Fatalf("blinker gen1 expected vertical %v, got %v", vertical, liveCells(gen1))
	}
	gen2 := stepN(t, g, nil, 2)
	if !sameCells(liveCells(gen2), horizontal) {
		t.Fatalf("blinker gen2 should return to horizontal, got %v", liveCells(gen2))
	}
}

func TestGliderTranslates(t *testing.T) {
	const n = 24
	g := NewGrid(n, n)
	glider := [][2]int{{6, 5}, {7, 6}, {5, 7}, {6, 7}, {7, 7}}
	setCells(g, glider)

	got := stepN(t, g, nil, 4)

	want := make([][2]int, len(glider))
	for i, c := range glider {
		want[i] = [2]int{c[0] + 1, c[1] + 1}
	}
	sort.Slice(want, func(i, j int) bool {
		if want[i][1] != want[j][1] {
			return want[i][1] < want[j][1]
		}
		return want[i][0] < want[j][0]
	})
	if !sameCells(liveCells(got), want) {
		t.Fatalf("glider after 4 gens expected %v, got %v", want, liveCells(got))
	}
}

func TestAllOnDiesOnTorus(t *testing.T) {
	g := NewGrid(16, 16)
	Seed(g, 1.0, 0)
	if g.Population() != 16*16 {
		t.Fatalf("expected fully alive grid, pop=%d", g.Population())
	}
	got := stepN(t, g, nil, 1)
	if got.Population() != 0 {
		t.Fatalf("all-on torus should be empty next gen, pop=%d", got.Population())
	}
}

func TestSeamBlinkerAcrossWrap(t *testing.T) {
	for _, w := range []int{65, 1000} {
		g := NewGrid(w, 9)
		y := 4
		setCells(g, [][2]int{{w - 1, y}, {0, y}, {1, y}})
		oracle := oracleFromGrid(g)

		got := stepN(t, g, nil, 1)
		oracle = oracle.step()
		if !oracle.equalsGrid(got) {
			t.Fatalf("w=%d seam blinker diverged from oracle: got %v", w, liveCells(got))
		}

		got2 := stepN(t, g, nil, 2)
		if !sameCells(liveCells(got2), liveCells(g)) {
			t.Fatalf("w=%d seam blinker not period-2: got %v", w, liveCells(got2))
		}
	}
}

func TestStepEquivalence(t *testing.T) {
	sizes := [][2]int{
		{8, 8}, {64, 64}, {65, 65}, {1000, 37}, {37, 1000}, {128, 65}, {2, 3}, {3, 2}, {130, 130},
	}
	pool := NewWorkerPool(4)
	defer pool.Close()

	for _, s := range sizes {
		w, h := s[0], s[1]
		g := NewGrid(w, h)
		r := rand.New(rand.NewPCG(uint64(w)*1000003+uint64(h), 0x1234))
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				if r.Float64() < 0.35 {
					g.Set(x, y, true)
				}
			}
		}

		cur := g.Clone()
		oracle := oracleFromGrid(cur)
		for gen := 0; gen < 8; gen++ {
			par := NewGrid(w, h)
			ser := NewGrid(w, h)
			if err := Step(cur, par, pool); err != nil {
				t.Fatalf("Step %dx%d: %v", w, h, err)
			}
			if err := StepSerial(cur, ser); err != nil {
				t.Fatalf("StepSerial %dx%d: %v", w, h, err)
			}
			if !par.Equal(ser) {
				t.Fatalf("%dx%d gen %d: parallel != serial", w, h, gen+1)
			}
			oracle = oracle.step()
			if !oracle.equalsGrid(par) {
				t.Fatalf("%dx%d gen %d: kernel != independent oracle (wrap/trailing bug?)", w, h, gen+1)
			}
			cur = par
		}
	}
}
