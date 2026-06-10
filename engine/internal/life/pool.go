package life

import "sync"

type WorkerPool struct {
	n    int
	jobs chan bandJob
	done sync.WaitGroup
}

type bandJob struct {
	cur, next        *Grid
	startRow, endRow int
}

func NewWorkerPool(n int) *WorkerPool {
	if n < 1 {
		n = 1
	}
	p := &WorkerPool{n: n, jobs: make(chan bandJob, n)}
	for i := 0; i < n; i++ {
		go p.worker()
	}
	return p
}

func (p *WorkerPool) Size() int { return p.n }

func (p *WorkerPool) worker() {
	for j := range p.jobs {
		computeBand(j.cur, j.next, j.startRow, j.endRow)
		p.done.Done()
	}
}

func (p *WorkerPool) run(cur, next *Grid) {
	h := cur.H
	bands := p.n
	if bands > h {
		bands = h
	}
	p.done.Add(bands)

	base := h / bands
	rem := h % bands
	start := 0
	for b := 0; b < bands; b++ {
		rows := base
		if b < rem {
			rows++
		}
		p.jobs <- bandJob{cur: cur, next: next, startRow: start, endRow: start + rows}
		start += rows
	}
	p.done.Wait()
}

func (p *WorkerPool) Close() {
	close(p.jobs)
}
