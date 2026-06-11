# b3s23-engine

[![Docker Pulls](https://img.shields.io/docker/pulls/mist941/b3s23-engine)](https://hub.docker.com/r/mist941/b3s23-engine)
[![Docker Image Version](https://img.shields.io/docker/v/mist941/b3s23-engine?sort=semver)](https://hub.docker.com/r/mist941/b3s23-engine/tags)
[![Image Size](https://img.shields.io/docker/image-size/mist941/b3s23-engine/latest)](https://hub.docker.com/r/mist941/b3s23-engine/tags)
[![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](https://github.com/mist941/b3s23-engine/blob/main/LICENSE)

Conway's Game of Life (B3/S23) as a self-contained service: a Go simulation
core, a REST + WebSocket streaming API, and a React/WebGL2 UI — all shipped in a single small distroless container.

## Quick start

```sh
docker run -d --name b3s23 -p 8050:8050 -v b3s23_data:/data mist941/b3s23-engine:latest
```

Open <http://localhost:8050> — the UI and the API are served from the same
port. The `/data` volume holds the SQLite database (plus its WAL sidecars), so
the simulation survives container restarts.

Or with Docker Compose (clone the repo, then):

```sh
docker compose up -d          # pulls the published image
docker compose up -d --build  # or build it from source
```

## License

[MIT](https://github.com/mist941/b3s23-engine/blob/main/LICENSE)
