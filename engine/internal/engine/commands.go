package engine

import (
	"context"
	"errors"
)

var (
	ErrRunning = errors.New("engine: operation not allowed while running")
	ErrClosed = errors.New("engine: simulation is shutting down")
)

type cmdKind int

const (
	cmdPlay cmdKind = iota
	cmdPause
	cmdStep
	cmdReset
	cmdReseed
	cmdSetProbability
	cmdSetSize
	cmdSetTickRate
	cmdSetStreamRate
	cmdSnapshot
	cmdShutdown
)

type command struct {
	kind         cmdKind
	prob         float64
	haveProb     bool
	seed         uint64
	haveSeed     bool
	width        int
	height       int
	tickHz       float64
	streamEveryN int
	reseed       bool
	reply        chan error
}

func (s *Simulation) send(ctx context.Context, c command) error {
	c.reply = make(chan error, 1)
	select {
	case s.cmds <- c:
	case <-ctx.Done():
		return ctx.Err()
	case <-s.quit:
		return ErrClosed
	}
	select {
	case err := <-c.reply:
		return err
	case <-ctx.Done():
		return ctx.Err()
	case <-s.quit:
		return ErrClosed
	}
}

func (s *Simulation) Play(ctx context.Context) error {
	return s.send(ctx, command{kind: cmdPlay})
}

func (s *Simulation) Pause(ctx context.Context) error {
	return s.send(ctx, command{kind: cmdPause})
}

func (s *Simulation) Step(ctx context.Context) error {
	return s.send(ctx, command{kind: cmdStep})
}

func (s *Simulation) Reset(ctx context.Context, seed *uint64) error {
	c := command{kind: cmdReset}
	if seed != nil {
		c.seed, c.haveSeed = *seed, true
	}
	return s.send(ctx, c)
}

func (s *Simulation) Reseed(ctx context.Context, prob *float64, seed *uint64) error {
	c := command{kind: cmdReseed}
	if prob != nil {
		c.prob, c.haveProb = *prob, true
	}
	if seed != nil {
		c.seed, c.haveSeed = *seed, true
	}
	return s.send(ctx, c)
}

func (s *Simulation) SetProbability(ctx context.Context, prob float64, reseed bool) error {
	return s.send(ctx, command{kind: cmdSetProbability, prob: prob, haveProb: true, reseed: reseed})
}

func (s *Simulation) SetSize(ctx context.Context, w, h int) error {
	return s.send(ctx, command{kind: cmdSetSize, width: w, height: h})
}

func (s *Simulation) SetTickRate(ctx context.Context, hz float64) error {
	return s.send(ctx, command{kind: cmdSetTickRate, tickHz: hz})
}

func (s *Simulation) SetStreamEveryN(ctx context.Context, n int) error {
	return s.send(ctx, command{kind: cmdSetStreamRate, streamEveryN: n})
}

func (s *Simulation) ForceSnapshot(ctx context.Context) error {
	return s.send(ctx, command{kind: cmdSnapshot})
}
