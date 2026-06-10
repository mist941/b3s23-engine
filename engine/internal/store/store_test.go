package store

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/mist941/b3s23-engine/engine/internal/engine"
	"github.com/mist941/b3s23-engine/engine/internal/gridcodec"
	"github.com/mist941/b3s23-engine/engine/internal/life"
)

func openTemp(t *testing.T, retain int) *Store {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.db")
	s, err := Open(path, retain)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func snapFor(t *testing.T, epoch, gen int64, reason string) (engine.Meta, engine.Snapshot) {
	t.Helper()
	g := life.NewGrid(64, 64)
	life.Seed(g, 0.1, uint64(epoch*1000+gen))
	blob, err := gridcodec.Encode(g, gridcodec.CodecRaw)
	if err != nil {
		t.Fatal(err)
	}
	m := engine.Meta{
		CurrentEpoch: epoch, Width: 64, Height: 64, Probability: 0.1, RngSeed: 7,
		TickHz: 10, StreamEveryN: 1, SnapshotEvery: 100, Generation: gen,
		StartTimeUnix: 1000, Running: false, UpdatedAtUnix: 2000 + gen,
	}
	sn := engine.Snapshot{
		Epoch: epoch, Generation: gen, Width: 64, Height: 64, WordsPerRow: g.WordsPerRow(),
		Codec: gridcodec.CodecRaw, Blob: blob, Checksum: gridcodec.Checksum(g),
		Reason: reason, CreatedAtUnix: 3000 + gen,
	}
	return m, sn
}

func TestColdLoadMeta(t *testing.T) {
	s := openTemp(t, 5)
	_, found, err := s.LoadMeta(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if found {
		t.Fatal("fresh database should have no meta row")
	}
}

func TestSaveAndLoadRoundTrip(t *testing.T) {
	s := openTemp(t, 5)
	ctx := context.Background()
	m, sn := snapFor(t, 1, 50, engine.ReasonPeriodic)
	if err := s.SaveSnapshot(ctx, m, sn); err != nil {
		t.Fatalf("SaveSnapshot: %v", err)
	}

	gotMeta, found, err := s.LoadMeta(ctx)
	if err != nil || !found {
		t.Fatalf("LoadMeta found=%v err=%v", found, err)
	}
	if gotMeta.Generation != 50 || gotMeta.CurrentEpoch != 1 {
		t.Fatalf("meta = %+v", gotMeta)
	}

	gotSnap, err := s.LoadLatest(ctx, 1)
	if err != nil || gotSnap == nil {
		t.Fatalf("LoadLatest err=%v snap=%v", err, gotSnap)
	}
	if gotSnap.Generation != 50 {
		t.Fatalf("snapshot generation = %d, want 50", gotSnap.Generation)
	}
	if _, err := gridcodec.Decode(gotSnap.Blob); err != nil {
		t.Fatalf("decode persisted blob: %v", err)
	}
}

func TestEpochResumeIgnoresStale(t *testing.T) {
	s := openTemp(t, 5)
	ctx := context.Background()

	for _, gen := range []int64{0, 100, 200} {
		m, sn := snapFor(t, 1, gen, engine.ReasonPeriodic)
		if err := s.SaveSnapshot(ctx, m, sn); err != nil {
			t.Fatalf("epoch1 gen%d: %v", gen, err)
		}
	}

	m2, sn2 := snapFor(t, 2, 0, engine.ReasonReset)
	if err := s.SaveSnapshot(ctx, m2, sn2); err != nil {
		t.Fatalf("epoch2 gen0: %v", err)
	}

	meta, _, err := s.LoadMeta(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if meta.CurrentEpoch != 2 || meta.Generation != 0 {
		t.Fatalf("after reset meta = epoch %d gen %d, want epoch 2 gen 0", meta.CurrentEpoch, meta.Generation)
	}

	latest, err := s.LoadLatest(ctx, meta.CurrentEpoch)
	if err != nil || latest == nil {
		t.Fatalf("LoadLatest(2) err=%v snap=%v", err, latest)
	}
	if latest.Generation != 0 || latest.Epoch != 2 {
		t.Fatalf("resume picked epoch %d gen %d, want the post-reset epoch 2 gen 0", latest.Epoch, latest.Generation)
	}

	stale, err := s.LoadLatest(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	if stale != nil {
		t.Fatalf("stale epoch 1 should be pruned, got gen %d", stale.Generation)
	}
}

func TestRetentionTrimsCurrentEpoch(t *testing.T) {
	s := openTemp(t, 3)
	ctx := context.Background()
	for _, gen := range []int64{0, 10, 20, 30, 40} {
		m, sn := snapFor(t, 1, gen, engine.ReasonPeriodic)
		if err := s.SaveSnapshot(ctx, m, sn); err != nil {
			t.Fatal(err)
		}
	}
	var count int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM snapshots WHERE epoch = 1`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 3 {
		t.Fatalf("retained %d snapshots, want 3", count)
	}
	latest, _ := s.LoadLatest(ctx, 1)
	if latest.Generation != 40 {
		t.Fatalf("latest generation = %d, want 40", latest.Generation)
	}
}

func TestResumeAfterReopen(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "reopen.db")

	s1, err := Open(path, 5)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	m, sn := snapFor(t, 1, 123, engine.ReasonPause)
	if err := s1.SaveSnapshot(ctx, m, sn); err != nil {
		t.Fatal(err)
	}
	if err := s1.Close(); err != nil {
		t.Fatal(err)
	}

	s2, err := Open(path, 5)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer s2.Close()
	meta, found, err := s2.LoadMeta(ctx)
	if err != nil || !found {
		t.Fatalf("reopened meta found=%v err=%v", found, err)
	}
	if meta.Generation != 123 {
		t.Fatalf("after reopen generation = %d, want 123", meta.Generation)
	}
}
