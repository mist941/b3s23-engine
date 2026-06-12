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
- **Drawing** (`src/render/GridCanvas.tsx`, `src/patterns.ts`): in Draw mode
  strokes are deduped per cell, Bresenham-interpolated between pointer events,
  and flushed as batched `setCells` WebSocket messages every 40 ms. Pattern
  stamps wrap toroidally, matching the engine topology; the ghost preview is a
  plain 2D overlay canvas so the WebGL pipeline stays untouched. The engine
  echoes a grid frame after every edit (even while paused), so drawn cells come
  back through the normal frame path — no optimistic rendering.
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

- **Tool**: Pan or Draw. In Draw mode: click toggles a cell, drag paints,
  Shift-drag erases, right/middle-drag pans, scroll always zooms
- **Pattern**: Freehand or a classic pattern (glider, LWSS, R-pentomino,
  acorn, diehard, pulsar, pentadecathlon, Gosper glider gun) — click stamps it
  centered under the cursor, with a ghost preview
- **Play / Pause / Step** (step is paused-only) · **Reset**
- **Probability** slider → **Reseed** (new epoch, generation 0)
- **Tick rate** (generations/second)
- **Size** (width × height) — pauses, starts a new epoch, reseeds
