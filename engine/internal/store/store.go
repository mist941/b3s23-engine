package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	_ "modernc.org/sqlite"

	"github.com/mist941/b3s23-engine/engine/internal/engine"
)

type Store struct {
	db             *sql.DB
	retainPerEpoch int
}

func Open(path string, retainPerEpoch int) (*Store, error) {
	dsn := "file:" + path +
		"?_pragma=journal_mode(WAL)" +
		"&_pragma=synchronous(NORMAL)" +
		"&_pragma=busy_timeout(5000)" +
		"&_pragma=foreign_keys(1)" +
		"&_txlock=immediate"

	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("store: open: %w", err)
	}

	db.SetMaxOpenConns(1)
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("store: ping: %w", err)
	}
	if err := migrate(db); err != nil {
		db.Close()
		return nil, err
	}
	if retainPerEpoch < 1 {
		retainPerEpoch = 1
	}
	return &Store{db: db, retainPerEpoch: retainPerEpoch}, nil
}

func (s *Store) LoadMeta(ctx context.Context) (*engine.Meta, bool, error) {
	const q = `SELECT current_epoch, width, height, probability, rng_seed, tick_hz,
	                  stream_every_n, snapshot_every, generation, start_time_unix,
	                  running, updated_at_unix
	           FROM meta WHERE id = 1`
	var (
		m       engine.Meta
		rngSeed int64
		running int
	)
	err := s.db.QueryRowContext(ctx, q).Scan(
		&m.CurrentEpoch, &m.Width, &m.Height, &m.Probability, &rngSeed, &m.TickHz,
		&m.StreamEveryN, &m.SnapshotEvery, &m.Generation, &m.StartTimeUnix,
		&running, &m.UpdatedAtUnix,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("store: load meta: %w", err)
	}
	m.RngSeed = uint64(rngSeed)
	m.Running = running != 0
	return &m, true, nil
}

func (s *Store) LoadLatest(ctx context.Context, epoch int64) (*engine.Snapshot, error) {
	const q = `SELECT seq, epoch, generation, width, height, words_per_row, codec,
	                  grid_blob, checksum, reason, created_at_unix
	           FROM snapshots WHERE epoch = ? ORDER BY seq DESC LIMIT 1`
	var (
		sn       engine.Snapshot
		codec    int
		checksum int64
	)
	err := s.db.QueryRowContext(ctx, q, epoch).Scan(
		&sn.Seq, &sn.Epoch, &sn.Generation, &sn.Width, &sn.Height, &sn.WordsPerRow,
		&codec, &sn.Blob, &checksum, &sn.Reason, &sn.CreatedAtUnix,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("store: load latest: %w", err)
	}
	sn.Codec = uint8(codec)
	sn.Checksum = uint32(checksum)
	return &sn, nil
}

func (s *Store) SaveSnapshot(ctx context.Context, m engine.Meta, sn engine.Snapshot) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("store: begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	const upsertMeta = `
	INSERT INTO meta (id, current_epoch, width, height, probability, rng_seed,
	                  tick_hz, stream_every_n, snapshot_every, generation,
	                  start_time_unix, running, updated_at_unix)
	VALUES (1, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	ON CONFLICT(id) DO UPDATE SET
	    current_epoch   = excluded.current_epoch,
	    width           = excluded.width,
	    height          = excluded.height,
	    probability     = excluded.probability,
	    rng_seed        = excluded.rng_seed,
	    tick_hz         = excluded.tick_hz,
	    stream_every_n  = excluded.stream_every_n,
	    snapshot_every  = excluded.snapshot_every,
	    generation      = excluded.generation,
	    start_time_unix = excluded.start_time_unix,
	    running         = excluded.running,
	    updated_at_unix = excluded.updated_at_unix`
	if _, err := tx.ExecContext(ctx, upsertMeta,
		m.CurrentEpoch, m.Width, m.Height, m.Probability, int64(m.RngSeed),
		m.TickHz, m.StreamEveryN, m.SnapshotEvery, m.Generation,
		m.StartTimeUnix, boolToInt(m.Running), m.UpdatedAtUnix,
	); err != nil {
		return fmt.Errorf("store: upsert meta: %w", err)
	}

	const insertSnap = `
	INSERT INTO snapshots (epoch, generation, width, height, words_per_row, codec,
	                       grid_blob, checksum, reason, created_at_unix)
	VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	if _, err := tx.ExecContext(ctx, insertSnap,
		sn.Epoch, sn.Generation, sn.Width, sn.Height, sn.WordsPerRow, int(sn.Codec),
		sn.Blob, int64(sn.Checksum), sn.Reason, sn.CreatedAtUnix,
	); err != nil {
		return fmt.Errorf("store: insert snapshot: %w", err)
	}

	if _, err := tx.ExecContext(ctx, `DELETE FROM snapshots WHERE epoch < ?`, m.CurrentEpoch); err != nil {
		return fmt.Errorf("store: prune epochs: %w", err)
	}
	const trim = `DELETE FROM snapshots
	              WHERE epoch = ?
	                AND seq NOT IN (SELECT seq FROM snapshots WHERE epoch = ? ORDER BY seq DESC LIMIT ?)`
	if _, err := tx.ExecContext(ctx, trim, m.CurrentEpoch, m.CurrentEpoch, s.retainPerEpoch); err != nil {
		return fmt.Errorf("store: trim epoch: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("store: commit: %w", err)
	}
	return nil
}

func (s *Store) Ping(ctx context.Context) error {
	return s.db.PingContext(ctx)
}

func (s *Store) Close() error {
	return s.db.Close()
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
