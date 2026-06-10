-- Singleton simulation metadata (id is always 1). Tracks lifetime (generation)
-- and start time continuously, plus current_epoch so resume is scoped to the
-- current run and can never select a stale pre-reset board.
CREATE TABLE IF NOT EXISTS meta (
    id              INTEGER PRIMARY KEY CHECK (id = 1),
    current_epoch   INTEGER NOT NULL DEFAULT 1,
    width           INTEGER NOT NULL CHECK (width  > 0),
    height          INTEGER NOT NULL CHECK (height > 0),
    probability     REAL    NOT NULL CHECK (probability >= 0.0 AND probability <= 1.0),
    rng_seed        INTEGER NOT NULL,
    tick_hz         REAL    NOT NULL DEFAULT 5.0,
    stream_every_n  INTEGER NOT NULL DEFAULT 1,
    snapshot_every  INTEGER NOT NULL DEFAULT 100,
    generation      INTEGER NOT NULL DEFAULT 0 CHECK (generation >= 0),
    start_time_unix INTEGER NOT NULL,
    running         INTEGER NOT NULL DEFAULT 0,
    updated_at_unix INTEGER NOT NULL
);

-- Full-state snapshots. The PRIMARY KEY is a monotonic surrogate (seq), NOT the
-- logical generation, so reusing generation=0 after a reset/reseed never
-- collides, and "latest" means most-recently-written within the current epoch.
CREATE TABLE IF NOT EXISTS snapshots (
    seq             INTEGER PRIMARY KEY AUTOINCREMENT,
    epoch           INTEGER NOT NULL,
    generation      INTEGER NOT NULL,
    width           INTEGER NOT NULL,
    height          INTEGER NOT NULL,
    words_per_row   INTEGER NOT NULL,
    codec           INTEGER NOT NULL DEFAULT 1,
    grid_blob       BLOB    NOT NULL,
    checksum        INTEGER NOT NULL,
    reason          TEXT    NOT NULL,
    created_at_unix INTEGER NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_snapshots_epoch_seq ON snapshots(epoch, seq DESC);
