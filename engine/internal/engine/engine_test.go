package engine_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/mist941/b3s23-engine/engine/internal/config"
	"github.com/mist941/b3s23-engine/engine/internal/engine"
)

type fakeStore struct {
	mu    sync.Mutex
	meta  *engine.Meta
	snaps []engine.Snapshot
}

func (f *fakeStore) LoadMeta(context.Context) (*engine.Meta, bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.meta == nil {
		return nil, false, nil
	}
	m := *f.meta
	return &m, true, nil
}

func (f *fakeStore) LoadLatest(_ context.Context, epoch int64) (*engine.Snapshot, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for i := len(f.snaps) - 1; i >= 0; i-- {
		if f.snaps[i].Epoch == epoch {
			s := f.snaps[i]
			return &s, nil
		}
	}
	return nil, nil
}

func (f *fakeStore) SaveSnapshot(_ context.Context, m engine.Meta, s engine.Snapshot) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	mc := m
	f.meta = &mc
	f.snaps = append(f.snaps, s)
	return nil
}

func (f *fakeStore) Ping(context.Context) error { return nil }
func (f *fakeStore) Close() error               { return nil }

func (f *fakeStore) lastReason() string {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.snaps) == 0 {
		return ""
	}
	return f.snaps[len(f.snaps)-1].Reason
}

type fakeBroadcaster struct {
	mu    sync.Mutex
	items []engine.BroadcastItem
}

func (b *fakeBroadcaster) Publish(it engine.BroadcastItem) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.items = append(b.items, it)
}

func (b *fakeBroadcaster) count(kind engine.FrameKind) int {
	b.mu.Lock()
	defer b.mu.Unlock()
	n := 0
	for _, it := range b.items {
		if it.Kind == kind {
			n++
		}
	}
	return n
}

func newSim(t *testing.T) (*engine.Simulation, *fakeStore, *fakeBroadcaster) {
	t.Helper()
	cfg := config.Default()
	cfg.Width, cfg.Height = 64, 64
	cfg.Probability = 0.3
	cfg.TickHz = 200
	if err := cfg.Validate(); err != nil {
		t.Fatal(err)
	}
	fs := &fakeStore{}
	fb := &fakeBroadcaster{}
	sim, err := engine.New(cfg, fs, fb, nil)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	sim.Start()
	t.Cleanup(func() { sim.Stop(context.Background()) })
	return sim, fs, fb
}

func TestColdSeedInitialState(t *testing.T) {
	sim, fs, fb := newSim(t)
	v := sim.CurrentView()
	if v == nil {
		t.Fatal("nil view after cold seed")
	}
	if v.Generation != 0 || v.State != engine.StatePaused {
		t.Fatalf("expected gen 0 paused, got gen %d state %v", v.Generation, v.State)
	}
	if v.Population == 0 {
		t.Fatal("cold seed at 30%% should have live cells")
	}
	if fs.lastReason() != engine.ReasonSeed {
		t.Fatalf("initial snapshot reason = %q, want seed", fs.lastReason())
	}
	if fb.count(engine.FrameGrid) == 0 {
		t.Fatal("expected an initial grid broadcast")
	}
}

func TestStepAdvancesGeneration(t *testing.T) {
	sim, _, _ := newSim(t)
	ctx := context.Background()
	for i := uint64(1); i <= 3; i++ {
		if err := sim.Step(ctx); err != nil {
			t.Fatalf("Step: %v", err)
		}
		if got := sim.CurrentView().Generation; got != i {
			t.Fatalf("after %d steps generation = %d", i, got)
		}
	}
}

func TestStepRejectedWhileRunning(t *testing.T) {
	sim, _, _ := newSim(t)
	ctx := context.Background()
	if err := sim.Play(ctx); err != nil {
		t.Fatal(err)
	}
	defer sim.Pause(ctx)
	if err := sim.Step(ctx); !errors.Is(err, engine.ErrRunning) {
		t.Fatalf("Step while running: got %v, want ErrRunning", err)
	}
}

func TestPlayAdvancesThenPause(t *testing.T) {
	sim, _, _ := newSim(t)
	ctx := context.Background()
	if err := sim.Play(ctx); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(2 * time.Second)
	for sim.CurrentView().Generation == 0 {
		if time.Now().After(deadline) {
			t.Fatal("generation did not advance while running")
		}
		time.Sleep(5 * time.Millisecond)
	}
	if err := sim.Pause(ctx); err != nil {
		t.Fatal(err)
	}
	g := sim.CurrentView().Generation
	time.Sleep(50 * time.Millisecond)
	if sim.CurrentView().Generation != g {
		t.Fatal("generation advanced after pause")
	}
	if sim.CurrentView().State != engine.StatePaused {
		t.Fatal("state not paused")
	}
}

func TestResetBumpsEpoch(t *testing.T) {
	sim, _, _ := newSim(t)
	ctx := context.Background()
	if err := sim.Step(ctx); err != nil {
		t.Fatal(err)
	}
	before := sim.CurrentView().Epoch
	if err := sim.Reset(ctx, nil); err != nil {
		t.Fatal(err)
	}
	v := sim.CurrentView()
	if v.Epoch != before+1 {
		t.Fatalf("epoch = %d, want %d", v.Epoch, before+1)
	}
	if v.Generation != 0 {
		t.Fatalf("generation after reset = %d, want 0", v.Generation)
	}
}

func TestSetSizeReallocates(t *testing.T) {
	sim, _, _ := newSim(t)
	ctx := context.Background()
	if err := sim.SetSize(ctx, 256, 128); err != nil {
		t.Fatalf("SetSize: %v", err)
	}
	v := sim.CurrentView()
	if v.Width != 256 || v.Height != 128 {
		t.Fatalf("size = %dx%d, want 256x128", v.Width, v.Height)
	}
	if v.Grid.W != 256 || v.Grid.H != 128 {
		t.Fatal("published grid geometry does not match view dimensions")
	}
}

func TestSetSizeRejectedWhileRunning(t *testing.T) {
	sim, _, _ := newSim(t)
	ctx := context.Background()
	if err := sim.Play(ctx); err != nil {
		t.Fatal(err)
	}
	defer sim.Pause(ctx)
	if err := sim.SetSize(ctx, 128, 128); !errors.Is(err, engine.ErrRunning) {
		t.Fatalf("SetSize while running: got %v, want ErrRunning", err)
	}
}

func TestSetCellsAppliesWhilePaused(t *testing.T) {
	sim, _, fb := newSim(t)
	ctx := context.Background()
	gridsBefore := fb.count(engine.FrameGrid)

	cells := []engine.Cell{
		{X: 1, Y: 1, Alive: true},
		{X: 2, Y: 1, Alive: true},
		{X: 3, Y: 1, Alive: false},
	}
	if err := sim.SetCells(ctx, cells); err != nil {
		t.Fatalf("SetCells: %v", err)
	}

	v := sim.CurrentView()
	if !v.Grid.Get(1, 1) || !v.Grid.Get(2, 1) {
		t.Fatal("cells were not set alive")
	}
	if v.Grid.Get(3, 1) {
		t.Fatal("cell was not cleared")
	}
	if fb.count(engine.FrameGrid) <= gridsBefore {
		t.Fatal("expected a grid broadcast after SetCells while paused")
	}
}

func TestSetCellsOutOfBoundsRejectedAtomically(t *testing.T) {
	sim, _, _ := newSim(t)
	ctx := context.Background()

	before := sim.CurrentView().Grid.Get(5, 5)
	cells := []engine.Cell{
		{X: 5, Y: 5, Alive: !before}, // valid: would flip the cell if applied
		{X: 64, Y: 0, Alive: true},   // out of bounds on a 64x64 grid
	}
	if err := sim.SetCells(ctx, cells); !errors.Is(err, engine.ErrCellOutOfBounds) {
		t.Fatalf("got %v, want ErrCellOutOfBounds", err)
	}

	// Force a republish of the live grid to observe any partial mutation.
	if err := sim.SetTickRate(ctx, 123); err != nil {
		t.Fatal(err)
	}
	if sim.CurrentView().Grid.Get(5, 5) != before {
		t.Fatal("rejected batch must not be applied partially")
	}
}

func TestSetCellsWhileRunning(t *testing.T) {
	sim, _, _ := newSim(t)
	ctx := context.Background()
	if err := sim.Play(ctx); err != nil {
		t.Fatal(err)
	}
	defer sim.Pause(ctx)
	if err := sim.SetCells(ctx, []engine.Cell{{X: 0, Y: 0, Alive: true}}); err != nil {
		t.Fatalf("SetCells while running: %v", err)
	}
}

func TestSetCellsBatchLimits(t *testing.T) {
	sim, _, _ := newSim(t)
	ctx := context.Background()

	if err := sim.SetCells(ctx, nil); err != nil {
		t.Fatalf("empty batch should be a no-op, got %v", err)
	}

	big := make([]engine.Cell, engine.MaxCellBatch+1)
	if err := sim.SetCells(ctx, big); !errors.Is(err, engine.ErrBatchTooLarge) {
		t.Fatalf("got %v, want ErrBatchTooLarge", err)
	}
}

func TestStopWritesFinalSnapshot(t *testing.T) {
	cfg := config.Default()
	cfg.Width, cfg.Height = 64, 64
	_ = cfg.Validate()
	fs := &fakeStore{}
	sim, err := engine.New(cfg, fs, &fakeBroadcaster{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	sim.Start()
	sim.Stop(context.Background())
	if fs.lastReason() != engine.ReasonStop {
		t.Fatalf("final snapshot reason = %q, want stop", fs.lastReason())
	}
}

func TestConcurrentReadsWhileRunning(t *testing.T) {
	sim, _, _ := newSim(t)
	ctx := context.Background()
	if err := sim.Play(ctx); err != nil {
		t.Fatal(err)
	}

	var wg sync.WaitGroup
	stop := make(chan struct{})

	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
				}
				if v := sim.CurrentView(); v != nil && v.Grid != nil {
					_ = v.Grid.Population() // reads every word of the published clone
					_ = v.Grid.Get(0, 0)
				}
			}
		}()
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 50; i++ {
			select {
			case <-stop:
				return
			default:
			}
			_ = sim.SetTickRate(ctx, 100+float64(i))
		}
	}()

	time.Sleep(300 * time.Millisecond)
	close(stop)
	wg.Wait()
	_ = sim.Pause(ctx)
}
