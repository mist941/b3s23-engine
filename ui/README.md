# b3s23-ui

React + WebGL2 front-end for the [B3S23 engine](../engine). It renders the live
grid on a `<canvas>` and drives the engine over its REST + WebSocket API.

## How it works

- **Rendering** (`src/render/`): the engine streams each generation as a
  bit-packed binary frame. The raw packed bytes are uploaded straight to a WebGL2
  `R8UI` integer texture and the bits are unpacked in the fragment shader
  (`texelFetch` + bit-shift) — no CPU-side unpacking. A single fullscreen triangle
  is drawn; each fragment resolves its cell from camera uniforms. Pan = drag,
  zoom = scroll (about the cursor), with fit-to-view on load/resize.
- **Networking** (`src/api/`, `src/ws/`): controls call the REST API; the
  WebSocket delivers `status` updates and `gen` header + binary grid pairs. The
  engine broadcasts the result of every control action, so the UI stays in sync
  without optimistic updates.

WebGL2 is required (`R8UI` / `usampler2D` / `texelFetch`); the canvas shows a
clear message if it is unavailable.

## Develop

```bash
npm install
npm run dev
```

The dev server proxies `/api` (REST + WebSocket) to the engine. Start the engine
first:

```bash
go -C ../engine run ./cmd/engined
```

Override the engine target for the dev proxy with `ENGINE_TARGET`, or point the
app at an absolute engine URL with `VITE_ENGINE_URL` (e.g. for a split
deployment).

## Build

```bash
npm run build      # tsc type-check + vite build -> dist/
npm run preview
```

## Docker

`docker compose up --build` (from the repo root) builds the UI inside the
image and bakes `dist/` into the engine container, which serves it at
http://localhost:8050 alongside the API (`engined -static /ui`).

## Controls

- **Play / Pause / Step** (step is paused-only) · **Reset** · **Snapshot**
- **Probability** slider → **Reseed** (new epoch, generation 0)
- **Tick rate** (generations/second) and **Stream every N** (WebSocket frame rate)
- **Size** (width × height) — pauses, starts a new epoch, reseeds
