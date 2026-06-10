package engine

import "context"

type RunState int

const (
	StateStopped RunState = iota
	StateRunning
	StatePaused
)

func (s RunState) String() string {
	switch s {
	case StateRunning:
		return "running"
	case StatePaused:
		return "paused"
	default:
		return "stopped"
	}
}

type Meta struct {
	CurrentEpoch  int64
	Width         int
	Height        int
	Probability   float64
	RngSeed       uint64
	TickHz        float64
	StreamEveryN  int
	SnapshotEvery int
	Generation    int64
	StartTimeUnix int64
	Running       bool
	UpdatedAtUnix int64
}

type Snapshot struct {
	Seq           int64
	Epoch         int64
	Generation    int64
	Width         int
	Height        int
	WordsPerRow   int
	Codec         uint8
	Blob          []byte
	Checksum      uint32
	Reason        string
	CreatedAtUnix int64
}

const (
	ReasonPeriodic = "periodic"
	ReasonPause    = "pause"
	ReasonStop     = "stop"
	ReasonReset    = "reset"
	ReasonReseed   = "reseed"
	ReasonManual   = "manual"
	ReasonResize   = "resize"
	ReasonSeed     = "seed"
)

type Store interface {
	LoadMeta(ctx context.Context) (m *Meta, found bool, err error)
	LoadLatest(ctx context.Context, epoch int64) (*Snapshot, error)
	SaveSnapshot(ctx context.Context, m Meta, s Snapshot) error
	Ping(ctx context.Context) error
	Close() error
}

type FrameKind int

const (
	FrameGrid FrameKind = iota
	FrameControl
)

type BroadcastItem struct {
	Kind       FrameKind
	Generation uint64
	Header     []byte
	Payload    []byte
}

type Broadcaster interface {
	Publish(item BroadcastItem)
}
