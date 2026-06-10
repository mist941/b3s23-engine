package life

import (
	"math"
	"testing"
)

func TestSeedDeterministic(t *testing.T) {
	a := NewGrid(256, 256)
	b := NewGrid(256, 256)
	Seed(a, 0.3, 42)
	Seed(b, 0.3, 42)
	if !a.Equal(b) {
		t.Fatal("same (prob, seed, dims) must produce identical grids")
	}
	c := NewGrid(256, 256)
	Seed(c, 0.3, 43)
	if a.Equal(c) {
		t.Fatal("different seed should (almost surely) differ")
	}
}

func TestSeedDensity(t *testing.T) {
	g := NewGrid(1024, 1024)
	Seed(g, 0.10, 7)
	got := float64(g.Population()) / float64(1024*1024)
	if math.Abs(got-0.10) > 0.01 {
		t.Fatalf("density = %.4f, want ~0.10", got)
	}
}

func TestSeedEdges(t *testing.T) {
	g := NewGrid(100, 100)
	Seed(g, 0, 1)
	if g.Population() != 0 {
		t.Fatalf("prob 0 should clear, pop=%d", g.Population())
	}
	Seed(g, 1, 1)
	if g.Population() != 100*100 {
		t.Fatalf("prob 1 should fill, pop=%d", g.Population())
	}
}
