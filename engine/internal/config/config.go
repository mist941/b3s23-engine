package config

import (
	"fmt"
	"math"
)

type Config struct {
	// Transport / storage.
	Addr   string // HTTP listen address, e.g. ":8050"
	DBPath string // SQLite database file path (keep on a local disk)

	// Initial simulation parameters.
	Width       int     // grid width in cells
	Height      int     // grid height in cells
	Probability float64 // seed life probability in [0,1]
	RngSeed     uint64  // deterministic seed for the initial cold board

	// Loop / streaming.
	TickHz         float64 // generations computed per second
	StreamEveryN   int     // broadcast every Nth generation (>=1)
	MaxWSFps       float64 // cap effective WS frame rate (0 = no cap)
	SnapshotEveryN int     // persist a full snapshot every N generations (>=1)

	// Guardrails.
	MaxCells       int  // hard upper bound on Width*Height (OOM guard)
	MinDim         int  // minimum width/height
	MaxDim         int  // maximum width/height
	MaxSubscribers int  // max concurrent WebSocket clients
	SendQueueSize  int  // per-client bounded send queue depth
	SnapshotRetain int  // snapshots kept per epoch (older pruned)
	FlateBlobs     bool // compress snapshot blobs with flate

	// CORS for the separate-origin WebGL UI ("*" for dev).
	CORSAllowOrigin string

	// Directory with the built UI to serve at /
	StaticDir string
}

func Default() Config {
	return Config{
		Addr:            ":8050",
		DBPath:          "engine.db",
		Width:           1024,
		Height:          1024,
		Probability:     0.10,
		RngSeed:         1,
		TickHz:          5,
		StreamEveryN:    1,
		MaxWSFps:        30,
		SnapshotEveryN:  100,
		MaxCells:        100_000_000,
		MinDim:          8,
		MaxDim:          16384,
		MaxSubscribers:  64,
		SendQueueSize:   4,
		SnapshotRetain:  5,
		FlateBlobs:      false,
		CORSAllowOrigin: "*",
		StaticDir:       "",
	}
}

func (c *Config) Validate() error {
	if c.Addr == "" {
		return fmt.Errorf("config: Addr must not be empty")
	}
	if c.DBPath == "" {
		return fmt.Errorf("config: DBPath must not be empty")
	}
	if c.MinDim < 1 {
		c.MinDim = 1
	}
	if c.MaxDim < c.MinDim {
		return fmt.Errorf("config: MaxDim %d < MinDim %d", c.MaxDim, c.MinDim)
	}
	if c.MaxCells < c.MinDim*c.MinDim {
		return fmt.Errorf("config: MaxCells %d too small", c.MaxCells)
	}
	if err := c.ValidateSize(c.Width, c.Height); err != nil {
		return err
	}
	c.Probability = ClampProbability(c.Probability)
	if c.TickHz <= 0 || math.IsNaN(c.TickHz) {
		return fmt.Errorf("config: TickHz must be > 0, got %v", c.TickHz)
	}
	if c.StreamEveryN < 1 {
		c.StreamEveryN = 1
	}
	if c.MaxWSFps < 0 || math.IsNaN(c.MaxWSFps) {
		return fmt.Errorf("config: MaxWSFps must be >= 0, got %v", c.MaxWSFps)
	}
	if c.SnapshotEveryN < 1 {
		c.SnapshotEveryN = 1
	}
	if c.MaxSubscribers < 1 {
		c.MaxSubscribers = 1
	}
	if c.SendQueueSize < 1 {
		c.SendQueueSize = 1
	}
	if c.SnapshotRetain < 1 {
		c.SnapshotRetain = 1
	}
	return nil
}

func (c *Config) ValidateSize(w, h int) error {
	if w < c.MinDim || h < c.MinDim {
		return fmt.Errorf("config: size %dx%d below minimum %d", w, h, c.MinDim)
	}
	if w > c.MaxDim || h > c.MaxDim {
		return fmt.Errorf("config: size %dx%d above maximum %d", w, h, c.MaxDim)
	}
	if w*h > c.MaxCells {
		return fmt.Errorf("config: size %dx%d = %d cells exceeds MaxCells %d", w, h, w*h, c.MaxCells)
	}
	return nil
}

func ClampProbability(p float64) float64 {
	switch {
	case math.IsNaN(p) || p < 0:
		return 0
	case p > 1:
		return 1
	default:
		return p
	}
}

func (c *Config) EffectiveStreamEveryN() int {
	n := c.StreamEveryN
	if n < 1 {
		n = 1
	}
	if c.MaxWSFps > 0 && c.TickHz > c.MaxWSFps {
		capN := int(math.Ceil(c.TickHz / c.MaxWSFps))
		if capN > n {
			n = capN
		}
	}
	return n
}
