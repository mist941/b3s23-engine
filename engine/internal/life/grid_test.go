package life

import "testing"

func TestGetSetPopulation(t *testing.T) {
	g := NewGrid(70, 5) // 70 forces a 2-word row with trailing bits
	if g.WordsPerRow() != 2 {
		t.Fatalf("wordsPerRow = %d, want 2", g.WordsPerRow())
	}
	cells := [][2]int{{0, 0}, {63, 0}, {64, 0}, {69, 4}, {1, 2}}
	for _, c := range cells {
		g.Set(c[0], c[1], true)
	}
	if got := g.Population(); got != len(cells) {
		t.Fatalf("population = %d, want %d", got, len(cells))
	}
	for _, c := range cells {
		if !g.Get(c[0], c[1]) {
			t.Fatalf("cell %v should be alive", c)
		}
	}
	g.Set(63, 0, false)
	if g.Get(63, 0) {
		t.Fatal("cell (63,0) should be dead after clearing")
	}
	if g.Population() != len(cells)-1 {
		t.Fatalf("population after clear = %d, want %d", g.Population(), len(cells)-1)
	}
}

func TestTrailingBitInvariant(t *testing.T) {
	for _, wh := range [][2]int{{65, 3}, {1000, 7}, {127, 127}} {
		g := NewGrid(wh[0], wh[1])
		Seed(g, 1.0, 0)
		if got, want := g.Population(), wh[0]*wh[1]; got != want {
			t.Fatalf("%dx%d full grid population = %d, want %d (trailing bits leaked?)", wh[0], wh[1], got, want)
		}
	}
}

func TestCloneIndependent(t *testing.T) {
	g := NewGrid(40, 40)
	g.Set(5, 5, true)
	c := g.Clone()
	if !g.Equal(c) {
		t.Fatal("clone should equal original")
	}
	c.Set(6, 6, true)
	if g.Equal(c) {
		t.Fatal("mutating clone must not affect original")
	}
	if g.Get(6, 6) {
		t.Fatal("original aliased clone's memory")
	}
}

func TestNewGridFromWordsRoundTrip(t *testing.T) {
	g := NewGrid(130, 9)
	Seed(g, 0.5, 99)
	g2, err := NewGridFromWords(g.W, g.H, g.Words())
	if err != nil {
		t.Fatalf("NewGridFromWords: %v", err)
	}
	if !g.Equal(g2) {
		t.Fatal("round-trip grid should be equal")
	}
	if _, err := NewGridFromWords(130, 9, g.Words()[:1]); err != ErrBadWordCount {
		t.Fatalf("expected ErrBadWordCount, got %v", err)
	}
}

func TestNextRule(t *testing.T) {
	// Exhaustive B3S23 truth table over all neighbour counts.
	for n := 0; n <= 8; n++ {
		wantDead := n == 3
		wantAlive := n == 2 || n == 3
		if Next(false, n) != wantDead {
			t.Errorf("dead cell with %d neighbours: got %v, want %v", n, Next(false, n), wantDead)
		}
		if Next(true, n) != wantAlive {
			t.Errorf("live cell with %d neighbours: got %v, want %v", n, Next(true, n), wantAlive)
		}
	}
}
