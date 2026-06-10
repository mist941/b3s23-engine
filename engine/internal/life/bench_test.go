package life

import (
	"runtime"
	"testing"
)

func BenchmarkStep1024(b *testing.B) {
	const n = 1024
	g := NewGrid(n, n)
	Seed(g, 0.3, 1)
	cur := g.Clone()
	next := NewGrid(n, n)
	pool := NewWorkerPool(runtime.GOMAXPROCS(0))
	defer pool.Close()

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = Step(cur, next, pool)
		cur, next = next, cur
	}
}

func BenchmarkStepSerial1024(b *testing.B) {
	const n = 1024
	g := NewGrid(n, n)
	Seed(g, 0.3, 1)
	cur := g.Clone()
	next := NewGrid(n, n)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = StepSerial(cur, next)
		cur, next = next, cur
	}
}
