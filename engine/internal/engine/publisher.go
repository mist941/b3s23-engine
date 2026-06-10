package engine

import (
	"sync/atomic"
	"time"

	"github.com/mist941/b3s23-engine/engine/internal/life"
)

type StateView struct {
	Grid         *life.Grid
	Generation   uint64
	Epoch        int64
	StartTime    time.Time
	Population   int
	Width        int
	Height       int
	State        RunState
	Probability  float64
	TickHz       float64
	StreamEveryN int
}

type publisher struct {
	v atomic.Pointer[StateView]
}

func (p *publisher) store(sv *StateView) { p.v.Store(sv) }
func (p *publisher) load() *StateView    { return p.v.Load() }
