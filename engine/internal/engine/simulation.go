package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"runtime"
	"sync"
	"time"

	"github.com/mist941/b3s23-engine/engine/internal/config"
	"github.com/mist941/b3s23-engine/engine/internal/gridcodec"
	"github.com/mist941/b3s23-engine/engine/internal/life"
)

type Simulation struct {
	cfg    config.Config
	store  Store
	bcast  Broadcaster
	pool   *life.WorkerPool
	logger *slog.Logger

	cmds    chan command
	snapReq chan snapshotJob
	quit    chan struct{}
	wg      sync.WaitGroup
	stopOne sync.Once

	pub       publisher
	blobCodec uint8

	cur, next     *life.Grid
	gen           uint64
	epoch         int64
	startTime     time.Time
	state         RunState
	prob          float64
	rngSeed       uint64
	tickHz        float64
	streamEveryN  int
	snapshotEvery int
	ticker        *time.Ticker
}

type snapshotJob struct {
	meta   Meta
	grid   *life.Grid
	reason string
}

func New(cfg config.Config, store Store, bcast Broadcaster, logger *slog.Logger) (*Simulation, error) {
	if logger == nil {
		logger = slog.Default()
	}
	s := &Simulation{
		cfg:     cfg,
		store:   store,
		bcast:   bcast,
		logger:  logger,
		pool:    life.NewWorkerPool(runtime.GOMAXPROCS(0)),
		cmds:    make(chan command, 32),
		snapReq: make(chan snapshotJob, 1),
		quit:    make(chan struct{}),
	}
	s.blobCodec = gridcodec.CodecRaw
	if cfg.FlateBlobs {
		s.blobCodec = gridcodec.CodecFlate
	}
	if err := s.initState(context.Background()); err != nil {
		s.pool.Close()
		return nil, err
	}
	s.ticker = time.NewTicker(s.interval())
	s.ticker.Stop()
	return s, nil
}

func (s *Simulation) initState(ctx context.Context) error {
	meta, found, err := s.store.LoadMeta(ctx)
	if err != nil {
		return fmt.Errorf("engine: load meta: %w", err)
	}
	if found {
		if s.tryResume(ctx, meta) {
			return nil
		}
	}
	return s.coldSeed(ctx)
}

func (s *Simulation) tryResume(ctx context.Context, meta *Meta) bool {
	snap, err := s.store.LoadLatest(ctx, meta.CurrentEpoch)
	if err != nil {
		s.logger.Warn("resume: load latest failed, cold seeding", "err", err)
		return false
	}
	if snap == nil {
		return false
	}
	g, err := gridcodec.Decode(snap.Blob)
	if err != nil {
		s.logger.Warn("resume: decode failed, cold seeding", "err", err)
		return false
	}

	if g.W != snap.Width || g.H != snap.Height || snap.Generation != meta.Generation {
		s.logger.Warn("resume: snapshot/meta mismatch, cold seeding",
			"snapGen", snap.Generation, "metaGen", meta.Generation)
		return false
	}

	s.cur = g
	s.next = life.NewGrid(g.W, g.H)
	s.gen = uint64(meta.Generation)
	s.epoch = meta.CurrentEpoch
	s.prob = meta.Probability
	s.rngSeed = meta.RngSeed
	s.tickHz = meta.TickHz
	s.streamEveryN = meta.StreamEveryN
	s.snapshotEvery = meta.SnapshotEvery
	s.startTime = time.Unix(meta.StartTimeUnix, 0)
	s.state = StatePaused // resume is intentionally paused

	s.logger.Info("resumed simulation", "generation", s.gen, "epoch", s.epoch,
		"size", fmt.Sprintf("%dx%d", g.W, g.H))
	pop := s.cur.Population()
	s.publish(pop)
	s.broadcastStatus()
	s.broadcastGrid(pop)
	return true
}

func (s *Simulation) coldSeed(ctx context.Context) error {
	s.epoch = 1
	s.cur = life.NewGrid(s.cfg.Width, s.cfg.Height)
	s.next = life.NewGrid(s.cfg.Width, s.cfg.Height)
	s.prob = s.cfg.Probability
	s.rngSeed = s.cfg.RngSeed
	s.tickHz = s.cfg.TickHz
	s.streamEveryN = s.cfg.EffectiveStreamEveryN()
	s.snapshotEvery = s.cfg.SnapshotEveryN
	s.gen = 0
	s.startTime = time.Now()
	s.state = StatePaused
	life.Seed(s.cur, s.prob, s.rngSeed)

	s.logger.Info("cold-seeded simulation", "size", fmt.Sprintf("%dx%d", s.cur.W, s.cur.H),
		"probability", s.prob)
	pop := s.cur.Population()
	s.publish(pop)
	s.broadcastStatus()
	s.broadcastGrid(pop)

	if err := s.writeSnapshot(ctx, snapshotJob{meta: s.currentMeta(), grid: s.cur.Clone(), reason: ReasonSeed}); err != nil {
		return fmt.Errorf("engine: initial snapshot: %w", err)
	}
	return nil
}

func (s *Simulation) Start() {
	s.wg.Add(2)
	go s.persistLoop()
	go s.loop()
}

func (s *Simulation) Stop(ctx context.Context) {
	s.stopOne.Do(func() {
		c := command{kind: cmdShutdown, reply: make(chan error, 1)}
		select {
		case s.cmds <- c:
			select {
			case <-c.reply:
			case <-ctx.Done():
			}
		case <-ctx.Done():
		}
		close(s.quit)    // reject any further commands
		close(s.snapReq) // let the persistence goroutine drain and exit
		s.wg.Wait()
		s.pool.Close()
	})
}

func (s *Simulation) CurrentView() *StateView { return s.pub.load() }

func (s *Simulation) Healthy(ctx context.Context) error { return s.store.Ping(ctx) }

func (s *Simulation) loop() {
	defer s.wg.Done()
	for {
		var tick <-chan time.Time
		if s.state == StateRunning {
			tick = s.ticker.C
		}
		select {
		case <-tick:
			s.advance(false)
		case c := <-s.cmds:
			if s.apply(c) {
				return
			}
		}
	}
}

func (s *Simulation) advance(manual bool) {
	if err := life.Step(s.cur, s.next, s.pool); err != nil {
		s.logger.Error("step", "err", err)
		return
	}
	s.cur, s.next = s.next, s.cur
	s.gen++
	pop := s.cur.Population()
	s.publish(pop)

	if manual || s.streamEveryN <= 1 || s.gen%uint64(s.streamEveryN) == 0 {
		s.broadcastGrid(pop)
	}
	if s.snapshotEvery >= 1 && s.gen%uint64(s.snapshotEvery) == 0 {
		s.requestSnapshot(ReasonPeriodic)
	}
}

func (s *Simulation) apply(c command) (exit bool) {
	switch c.kind {
	case cmdPlay:
		if s.state != StateRunning {
			s.state = StateRunning
			s.ticker.Reset(s.interval())
			s.publishCurrent()
			s.broadcastStatus()
		}
		c.reply <- nil

	case cmdPause:
		if s.state == StateRunning {
			s.state = StatePaused
			s.ticker.Stop()
			s.publishCurrent()
			s.broadcastStatus()
			s.requestSnapshot(ReasonPause)
		}
		c.reply <- nil

	case cmdStep:
		if s.state == StateRunning {
			c.reply <- ErrRunning
		} else {
			s.advance(true)
			c.reply <- nil
		}

	case cmdReset:
		seed := newSeed()
		if c.haveSeed {
			seed = c.seed
		}
		s.reseedNewEpoch(s.prob, seed, ReasonReset)
		c.reply <- nil

	case cmdReseed:
		prob := s.prob
		if c.haveProb {
			prob = config.ClampProbability(c.prob)
		}
		seed := newSeed()
		if c.haveSeed {
			seed = c.seed
		}
		s.reseedNewEpoch(prob, seed, ReasonReseed)
		c.reply <- nil

	case cmdSetProbability:
		s.prob = config.ClampProbability(c.prob)
		if c.reseed {
			s.reseedNewEpoch(s.prob, newSeed(), ReasonReseed)
		} else {
			s.publishCurrent()
			s.broadcastStatus()
		}
		c.reply <- nil

	case cmdSetSize:
		if s.state == StateRunning {
			c.reply <- ErrRunning
			break
		}
		if err := s.cfg.ValidateSize(c.width, c.height); err != nil {
			c.reply <- err
			break
		}
		s.resizeNewEpoch(c.width, c.height)
		c.reply <- nil

	case cmdSetTickRate:
		if c.tickHz > 0 {
			s.tickHz = c.tickHz
			if s.state == StateRunning {
				s.ticker.Reset(s.interval())
			}
			s.publishCurrent()
			s.broadcastStatus()
		}
		c.reply <- nil

	case cmdSetStreamRate:
		if c.streamEveryN >= 1 {
			s.streamEveryN = c.streamEveryN
			s.publishCurrent()
			s.broadcastStatus()
		}
		c.reply <- nil

	case cmdSnapshot:
		s.requestSnapshot(ReasonManual)
		c.reply <- nil

	case cmdShutdown:
		s.state = StatePaused
		s.ticker.Stop()
		s.requestSnapshot(ReasonStop)
		c.reply <- nil
		return true
	}
	return false
}

func (s *Simulation) reseedNewEpoch(prob float64, seed uint64, reason string) {
	s.epoch++
	s.prob = prob
	s.rngSeed = seed
	s.gen = 0
	s.startTime = time.Now()
	life.Seed(s.cur, prob, seed)
	pop := s.cur.Population()
	s.publish(pop)
	s.broadcastStatus()
	s.broadcastGrid(pop)
	s.requestSnapshot(reason)
}

func (s *Simulation) resizeNewEpoch(w, h int) {
	s.epoch++
	s.rngSeed = newSeed()
	s.cur = life.NewGrid(w, h)
	s.next = life.NewGrid(w, h)
	s.gen = 0
	s.startTime = time.Now()
	life.Seed(s.cur, s.prob, s.rngSeed)
	pop := s.cur.Population()
	s.publish(pop)
	s.broadcastStatus()
	s.broadcastGrid(pop)
	s.requestSnapshot(ReasonResize)
}

func (s *Simulation) publish(pop int) {
	s.pub.store(&StateView{
		Grid:         s.cur.Clone(),
		Generation:   s.gen,
		Epoch:        s.epoch,
		StartTime:    s.startTime,
		Population:   pop,
		Width:        s.cur.W,
		Height:       s.cur.H,
		State:        s.state,
		Probability:  s.prob,
		TickHz:       s.tickHz,
		StreamEveryN: s.streamEveryN,
	})
}

func (s *Simulation) publishCurrent() { s.publish(s.cur.Population()) }

func (s *Simulation) broadcastGrid(pop int) {
	blob, err := gridcodec.Encode(s.cur, gridcodec.CodecRaw)
	if err != nil {
		s.logger.Error("encode grid frame", "err", err)
		return
	}
	header, _ := json.Marshal(genHeader{
		Type: "gen", Generation: s.gen, Epoch: s.epoch, Population: pop,
	})
	s.bcast.Publish(BroadcastItem{Kind: FrameGrid, Generation: s.gen, Header: header, Payload: blob})
}

func (s *Simulation) broadcastStatus() {
	header, _ := json.Marshal(s.statusMsg())
	s.bcast.Publish(BroadcastItem{Kind: FrameControl, Generation: s.gen, Header: header})
}

func (s *Simulation) statusMsg() statusMsg {
	return statusMsg{
		Type:         "status",
		State:        s.state.String(),
		Generation:   s.gen,
		Epoch:        s.epoch,
		Width:        s.cur.W,
		Height:       s.cur.H,
		Probability:  s.prob,
		TickHz:       s.tickHz,
		StreamEveryN: s.streamEveryN,
		Population:   s.cur.Population(),
		StartTime:    s.startTime.UTC().Format(time.RFC3339),
	}
}

func (s *Simulation) currentMeta() Meta {
	return Meta{
		CurrentEpoch:  s.epoch,
		Width:         s.cur.W,
		Height:        s.cur.H,
		Probability:   s.prob,
		RngSeed:       s.rngSeed,
		TickHz:        s.tickHz,
		StreamEveryN:  s.streamEveryN,
		SnapshotEvery: s.snapshotEvery,
		Generation:    int64(s.gen),
		StartTimeUnix: s.startTime.Unix(),
		Running:       s.state == StateRunning,
		UpdatedAtUnix: time.Now().Unix(),
	}
}

func (s *Simulation) requestSnapshot(reason string) {
	job := snapshotJob{meta: s.currentMeta(), grid: s.cur.Clone(), reason: reason}
	for {
		select {
		case s.snapReq <- job:
			return
		default:
			select {
			case <-s.snapReq: // drop the stale pending job, retry with the newest
			default:
			}
		}
	}
}

func (s *Simulation) persistLoop() {
	defer s.wg.Done()
	for job := range s.snapReq {
		if err := s.writeSnapshot(context.Background(), job); err != nil {
			s.logger.Error("snapshot write", "err", err, "reason", job.reason)
		}
	}
}

func (s *Simulation) writeSnapshot(ctx context.Context, job snapshotJob) error {
	blob, err := gridcodec.Encode(job.grid, s.blobCodec)
	if err != nil {
		return fmt.Errorf("encode snapshot: %w", err)
	}
	sn := Snapshot{
		Epoch:         job.meta.CurrentEpoch,
		Generation:    job.meta.Generation,
		Width:         job.grid.W,
		Height:        job.grid.H,
		WordsPerRow:   job.grid.WordsPerRow(),
		Codec:         s.blobCodec,
		Blob:          blob,
		Checksum:      gridcodec.Checksum(job.grid),
		Reason:        job.reason,
		CreatedAtUnix: time.Now().Unix(),
	}
	return s.store.SaveSnapshot(ctx, job.meta, sn)
}

func (s *Simulation) interval() time.Duration {
	if s.tickHz <= 0 {
		return time.Second
	}
	d := time.Duration(float64(time.Second) / s.tickHz)
	if d < time.Millisecond {
		d = time.Millisecond // cap at 1000 ticks/s to keep the ticker sane
	}
	return d
}

func newSeed() uint64 { return rand.Uint64() }

type genHeader struct {
	Type       string `json:"type"`
	Generation uint64 `json:"generation"`
	Epoch      int64  `json:"epoch"`
	Population int    `json:"population"`
}

type statusMsg struct {
	Type         string  `json:"type"`
	State        string  `json:"state"`
	Generation   uint64  `json:"generation"`
	Epoch        int64   `json:"epoch"`
	Width        int     `json:"width"`
	Height       int     `json:"height"`
	Probability  float64 `json:"probability"`
	TickHz       float64 `json:"tickHz"`
	StreamEveryN int     `json:"streamEveryN"`
	Population   int     `json:"population"`
	StartTime    string  `json:"startTime"`
}
