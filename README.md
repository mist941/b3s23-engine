# b3s23-engine

[![CI](https://github.com/mist941/b3s23-engine/actions/workflows/ci.yml/badge.svg)](https://github.com/mist941/b3s23-engine/actions/workflows/ci.yml)
[![Docker Pulls](https://img.shields.io/docker/pulls/mist941/b3s23-engine)](https://hub.docker.com/r/mist941/b3s23-engine)
[![Docker Image Version](https://img.shields.io/docker/v/mist941/b3s23-engine?sort=semver)](https://hub.docker.com/r/mist941/b3s23-engine/tags)
[![Image Size](https://img.shields.io/docker/image-size/mist941/b3s23-engine/latest)](https://hub.docker.com/r/mist941/b3s23-engine/tags)
[![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](https://github.com/mist941/b3s23-engine/blob/main/LICENSE)

Conway's Game of Life (B3/S23) as a self-contained service: a Go simulation
core with a REST + WebSocket streaming API and a React/WebGL2 UI — all shipped
in a single small distroless container. State lives in SQLite, so the world
survives restarts.

## Demo

![B3S23 Engine — drawing cells, stamping patterns and streaming the live simulation](https://raw.githubusercontent.com/mist941/b3s23-engine/main/docs/demo.gif)

<sub>Sped up ~12× from a live session.</sub>

## Features

- **Interactive editing** — draw cells with the mouse (paint, erase, toggle)
  or stamp classic patterns (glider, LWSS, R-pentomino, acorn, diehard,
  pulsar, pentadecathlon, Gosper glider gun) with a live ghost preview.
- **WebGL2 renderer** — bit-packed frames are unpacked on the GPU; smooth pan
  and zoom even on large grids.
- **REST + WebSocket API** — full control surface (play/pause/step, reseed,
  resize, tick rate, cell editing) plus binary frame streaming with
  per-client backpressure.
- **Persistent** — the grid is snapshotted to SQLite and restored on restart.
- **Compact engine** — bit-packed toroidal grid (1 bit per cell) up to
  16384×16384, stepped in parallel across CPU cores.
- **Easy to run** — one small distroless Docker image (amd64 + arm64), no
  configuration required, health check included.

## Run

### Docker

```sh
docker run -d --name b3s23 -p 8050:8050 -v b3s23_data:/data mist941/b3s23-engine:latest
```

Open <http://localhost:8050> — the UI and the API are served from the same
port. The `/data` volume holds the SQLite database (plus its WAL sidecars), so
the simulation survives container restarts.

### Docker Compose

Clone the repo, then:

```sh
docker compose up -d          # pulls the published image
docker compose up -d --build  # or build it from source
```

### From source

Engine (Go ≥ 1.26, no CGO):

```sh
cd engine
go run ./cmd/engined    # API on :8050
go test ./...           # -race needs gcc; CI runs it on Linux
```

UI dev server (Node ≥ 24) with hot reload, proxying `/api` to the engine:

```sh
cd ui
npm install
npm run dev
```

Or serve the built UI straight from the engine binary:

```sh
cd ui && npm install && npm run build
cd ../engine && go run ./cmd/engined -static ../ui/dist
```

Run `engined -help` for all flags (grid size, tick rate, DB path, CORS, …).

## License

[MIT](https://github.com/mist941/b3s23-engine/blob/main/LICENSE)
