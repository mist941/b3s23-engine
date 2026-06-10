# b3s23-engine

A high-performance **B3S23 engine** — Conway's Game of Life (**B**orn on exactly
**3** live neighbours, **S**urvives on **2** or **3**) — written in Go.

It simulates a toroidal grid (default **1024×1024**, seeded at **10%**), advances
generations on a multithreaded server-side loop with play / pause / step / reset,
streams each generation over **WebSocket**, exposes an HTTP API to reconfigure and
read the system, and persists to **SQLite** so it resumes after a restart.

> This repository currently contains the **backend engine** only. A separate
> WebGL/canvas Web UI will consume this API.

## Quick start

```bash
cd engine
go run ./cmd/engined                 # listens on :8050, DB at ./engine.db

# or build a static, CGO-free binary
CGO_ENABLED=0 go build -o engined ./cmd/engined
./engined -addr :8050 -db ./engine.db -width 1024 -height 1024 -prob 0.10
```

With Docker:

```bash
docker compose up --build            # UI + engine API on http://localhost:8050
```

Drive it:

```bash
curl localhost:8050/api/v1/status
curl -X POST localhost:8050/api/v1/play
curl -X POST localhost:8050/api/v1/pause
curl -X POST localhost:8050/api/v1/step
curl localhost:8050/api/v1/grid -o grid.bin    # binary packed frame
```

## Architecture

Dependency-inward, single-format design. The grid is a **bit-packed `[]uint64`**
(1 bit/cell, 128 KiB at 1024²) that serves as the in-memory compute format, the
WebSocket wire payload, **and** the SQLite blob — one format, zero re-encoding.

```
engine/
  cmd/engined/        composition root: wiring + graceful shutdown
  internal/
    life/             pure simulation core (no I/O): Grid, B3S23 rule,
                      parallel toroidal Step, worker pool, deterministic seeding
    gridcodec/        the packed wire/disk format (header + bits + CRC32C)
    engine/           the simulation actor: owns the grid, command loop,
                      lock-free StateView publisher, snapshot scheduler
    store/            SQLite persistence (modernc.org/sqlite, pure Go)
    httpapi/          net/http ServeMux REST handlers
    wsapi/            coder/websocket hub + per-client backpressure
    config/           defaults + validation
```

Key properties:

- **Multithreaded compute.** A generation is computed in parallel: rows are split
  into bands across a persistent `GOMAXPROCS` worker pool, each writing disjoint
  rows of the back buffer while reading only the (immutable) front buffer — no
  locks. Boundaries wrap **toroidally** on all four edges.
- **Single-owner concurrency.** One actor goroutine owns all mutable state and
  applies every change (play/pause/resize/reseed/…) as a command at a generation
  boundary. A second goroutine is the sole SQLite writer. Reads (`GET /grid`,
  `GET /status`, WS hello) come from an `atomic.Pointer` to an **immutable
  write-once clone**, so they are lock-free and race-free (`go test -race` clean).
- **Resumable persistence.** Snapshots are written every N generations and on
  pause/stop/reset, in one transaction with the metadata. A monotonic `seq` +
  `epoch` scheme means a restart resumes the *current* run's latest board and can
  never resurrect a stale pre-reset board.

## HTTP API (`/api/v1`)

| Method | Path | Description |
|---|---|---|
| `GET` | `/status` | State, generation, epoch, size, probability, tick rate, population, start time |
| `GET` | `/grid` | Current grid. Binary packed (default) or JSON base64 (`Accept: application/json`) |
| `POST` | `/play` | Start/resume the loop |
| `POST` | `/pause` | Pause the loop (+ snapshot) |
| `POST` | `/step` | Advance one generation (paused only; `409` if running) |
| `POST` | `/reset` | New epoch, generation 0, reseed. Body: `{"rngSeed"?: int}` |
| `POST` | `/reseed` | Change initial data. Body: `{"probability"?: float, "rngSeed"?: int}` |
| `PUT` | `/config/probability` | `{"probability": float, "reseed"?: bool}` |
| `PUT` | `/config/size` | `{"width": int, "height": int}` (`409` if running, `400` if out of bounds) |
| `PUT` | `/config/tickrate` | `{"tickHz": float}` |
| `PUT` | `/config/streamrate` | `{"streamEveryN": int}` (decouple WS FPS from tick rate) |
| `POST` | `/snapshot` | Force an immediate snapshot |
| `GET` | `/healthz` | Liveness + DB reachability |
| `GET` | `/ws` | WebSocket upgrade (`503` past the subscriber cap) |

Most endpoints return the resulting `status` object.

## WebSocket protocol (`GET /api/v1/ws`)

On connect the server sends a JSON `status` control frame (acting as *hello*) then
the current grid. Per streamed generation it sends a JSON `gen` header
(`{type, generation, epoch, population}`) immediately followed by one **binary**
frame — the packed grid, ready to upload to a WebGL texture.

- A generation's header + binary are one atomic unit; under backpressure only grid
  frames coalesce (newest board wins), while control frames (status/resize) are
  never dropped — a client that falls too far behind is disconnected rather than
  shown a board under the wrong dimensions.
- Clients may send control messages (`{"type":"play"|"pause"|"step"}`,
  `{"type":"setRate","tickHz":…}`, `{"type":"setStreamRate","streamEveryN":…}`),
  routed through the same command path as HTTP.

## Configuration (flags)

| Flag | Default | Description |
|---|---|---|
| `-addr` | `:8050` | HTTP listen address |
| `-db` | `engine.db` | SQLite path (keep on local disk) |
| `-width` / `-height` | `1024` | Initial grid size |
| `-prob` | `0.10` | Seed life probability |
| `-seed` | `1` | RNG seed for the initial cold board |
| `-tick` | `5` | Generations per second |
| `-stream` | `1` | Broadcast every Nth generation |
| `-maxfps` | `30` | Cap effective WS frame rate (0 = no cap) |
| `-snapshot` | `100` | Snapshot every N generations |
| `-retain` | `5` | Snapshots kept per epoch |
| `-maxcells` | `100000000` | Hard cap on width×height |
| `-maxsubs` | `64` | Max concurrent WebSocket clients |
| `-flate` | `false` | Compress snapshot blobs with flate |
| `-cors` | `*` | `Access-Control-Allow-Origin` |
| `-static` | _(empty)_ | Directory with the built UI to serve at `/` (empty = API only) |
| `-log` | `info` | `debug` / `info` / `warn` / `error` |

## Testing

```bash
cd engine
go test ./...            # unit + integration tests
go test -race ./...      # requires a C toolchain (CI runs this on Linux)
```

Coverage includes the B3S23 rule and known patterns (blinker, glider, still
lifes), parallel-vs-serial-vs-independent-oracle equivalence across non-64-aligned
widths (the toroidal seam), codec round-trips and corruption rejection, SQLite
epoch-scoped resume, the HTTP endpoints, and WebSocket frame atomicity /
backpressure / reaping.

## License

MIT — see [LICENSE](LICENSE).
