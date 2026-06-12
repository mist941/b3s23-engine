import {
  memo,
  useEffect,
  useImperativeHandle,
  useRef,
  useState,
  type Ref,
} from "react";

import { createRenderer, type Camera, type RgbColors } from "./webgl";
import type { GridFrame } from "../grid/decode";
import type { CellWrite } from "../ws/useEngineSocket";
import type { Pattern } from "../patterns";

export interface GridColors {
  alive: string;
  dead: string;
  bg: string;
  grid: string;
}

export interface GridHandle {
  pushFrame: (frame: GridFrame) => void;
}

export type Tool = "pan" | "draw";

interface Props {
  colors: GridColors;
  gridLines?: boolean;
  tool?: Tool;
  pattern?: Pattern | null;
  onPaintCells?: (cells: CellWrite[]) => void;
  onFps?: (fps: number) => void;
  ref?: Ref<GridHandle>;
}

const MIN_CELL = 0.02;
const MAX_CELL = 80;

function hexToRgb(hex: string): [number, number, number] {
  const h = hex.replace("#", "");
  const n = parseInt(h.length === 3 ? h.replace(/(.)/g, "$1$1") : h, 16);
  return [((n >> 16) & 255) / 255, ((n >> 8) & 255) / 255, (n & 255) / 255];
}

function toRgbColors(c: GridColors): RgbColors {
  return {
    alive: hexToRgb(c.alive),
    dead: hexToRgb(c.dead),
    bg: hexToRgb(c.bg),
    grid: hexToRgb(c.grid),
  };
}

function GridCanvasImpl({
  colors,
  gridLines = true,
  tool = "pan",
  pattern = null,
  onPaintCells,
  onFps,
  ref,
}: Props) {
  const canvasRef = useRef<HTMLCanvasElement | null>(null);
  const overlayRef = useRef<HTMLCanvasElement | null>(null);
  const frameRef = useRef<GridFrame | null>(null);
  const uploadPendingRef = useRef(false);
  const cameraRef = useRef<Camera>({ cellSize: 1, originX: 0, originY: 0 });
  const dirtyRef = useRef(true);
  const fittedDimsRef = useRef<{ w: number; h: number } | null>(null);
  const gridLinesRef = useRef(gridLines);
  gridLinesRef.current = gridLines;
  const toolRef = useRef<Tool>(tool);
  toolRef.current = tool;
  const patternRef = useRef<Pattern | null>(pattern);
  patternRef.current = pattern;
  const onPaintCellsRef = useRef(onPaintCells);
  onPaintCellsRef.current = onPaintCells;
  const ghostCellRef = useRef<{ x: number; y: number } | null>(null);
  const ghostFill = colors.alive + "59";

  const [error, setError] = useState<string | null>(null);

  useImperativeHandle(
    ref,
    () => ({
      pushFrame: (frame: GridFrame) => {
        frameRef.current = frame;
        uploadPendingRef.current = true;
        dirtyRef.current = true;
      },
    }),
    [],
  );

  useEffect(() => {
    const canvas = canvasRef.current;
    if (!canvas) return;

    const renderer = createRenderer(canvas, toRgbColors(colors));
    if (!renderer) {
      setError(
        "WebGL2 is required to render the grid, but it is not available in this browser.",
      );
      return;
    }

    const fit = () => {
      const f = frameRef.current;
      if (!f || canvas.width === 0 || canvas.height === 0) return;
      const cell = Math.min(canvas.width / f.width, canvas.height / f.height);
      cameraRef.current = {
        cellSize: cell,
        originX: (canvas.width - f.width * cell) / 2,
        originY: (canvas.height - f.height * cell) / 2,
      };
      dirtyRef.current = true;
    };

    const resizeToDisplay = (): boolean => {
      const dpr = window.devicePixelRatio || 1;
      const w = Math.max(1, Math.round(canvas.clientWidth * dpr));
      const h = Math.max(1, Math.round(canvas.clientHeight * dpr));
      if (canvas.width !== w || canvas.height !== h) {
        canvas.width = w;
        canvas.height = h;
        return true;
      }
      return false;
    };

    let raf = 0;
    let frames = 0;
    let fpsClock = performance.now();
    let ghostVisible = false;

    const drawGhost = () => {
      const overlay = overlayRef.current;
      if (!overlay) return;
      const p = patternRef.current;
      const g = ghostCellRef.current;
      const f = frameRef.current;
      const show = p && g && f && toolRef.current === "draw";
      if (!show && !ghostVisible) return;
      if (overlay.width !== canvas.width || overlay.height !== canvas.height) {
        overlay.width = canvas.width;
        overlay.height = canvas.height;
      }
      const ctx = overlay.getContext("2d");
      if (!ctx) return;
      ctx.clearRect(0, 0, overlay.width, overlay.height);
      ghostVisible = false;
      if (!show) return;
      const cam = cameraRef.current;
      const ox = g.x - (p.w >> 1);
      const oy = g.y - (p.h >> 1);
      const size = Math.max(cam.cellSize, 1);
      ctx.fillStyle = ghostFill;
      for (const [px, py] of p.cells) {
        const x = (((ox + px) % f.width) + f.width) % f.width;
        const y = (((oy + py) % f.height) + f.height) % f.height;
        ctx.fillRect(
          cam.originX + x * cam.cellSize,
          cam.originY + y * cam.cellSize,
          size,
          size,
        );
      }
      ghostVisible = true;
    };

    const loop = () => {
      raf = requestAnimationFrame(loop);

      if (resizeToDisplay()) {
        if (!fittedDimsRef.current) fit();
        dirtyRef.current = true;
      }

      if (uploadPendingRef.current && frameRef.current) {
        renderer.upload(frameRef.current);
        uploadPendingRef.current = false;
        const f = frameRef.current;
        const fd = fittedDimsRef.current;
        if (!fd || fd.w !== f.width || fd.h !== f.height) {
          fit();
          fittedDimsRef.current = { w: f.width, h: f.height };
        }
        dirtyRef.current = true;
      }

      if (dirtyRef.current && renderer.hasFrame()) {
        renderer.draw(cameraRef.current, gridLinesRef.current);
        dirtyRef.current = false;
      }

      drawGhost();

      frames++;
      const now = performance.now();
      if (now - fpsClock >= 1000) {
        onFps?.((frames * 1000) / (now - fpsClock));
        frames = 0;
        fpsClock = now;
      }
    };
    raf = requestAnimationFrame(loop);

    return () => {
      cancelAnimationFrame(raf);
      renderer.dispose();
    };
  }, []);

  useEffect(() => {
    const canvas = canvasRef.current;
    if (!canvas) return;
    const dpr = () => window.devicePixelRatio || 1;
    let dragging = false;
    let lastX = 0;
    let lastY = 0;

    let painting = false;
    let erase = false;
    let moved = false;
    let downCell: { x: number; y: number } | null = null;
    let lastCell: { x: number; y: number } | null = null;
    let stampDown: { x: number; y: number } | null = null;
    const stroke = new Map<string, 0 | 1>();
    let flushTimer = 0;

    const screenToCell = (e: PointerEvent) => {
      const f = frameRef.current;
      if (!f) return null;
      const rect = canvas.getBoundingClientRect();
      const cam = cameraRef.current;
      const x = Math.floor(
        ((e.clientX - rect.left) * dpr() - cam.originX) / cam.cellSize,
      );
      const y = Math.floor(
        ((e.clientY - rect.top) * dpr() - cam.originY) / cam.cellSize,
      );
      if (x < 0 || x >= f.width || y < 0 || y >= f.height) return null;
      return { x, y };
    };

    const cellAlive = (x: number, y: number): boolean => {
      const f = frameRef.current;
      if (!f) return false;
      return ((f.payload[y * f.rowBytes + (x >> 3)] >> (x & 7)) & 1) === 1;
    };

    const flushStroke = () => {
      if (flushTimer) {
        window.clearTimeout(flushTimer);
        flushTimer = 0;
      }
      if (stroke.size === 0) return;
      const cells: CellWrite[] = [];
      stroke.forEach((alive, key) => {
        const i = key.indexOf(",");
        cells.push([Number(key.slice(0, i)), Number(key.slice(i + 1)), alive]);
      });
      stroke.clear();
      onPaintCellsRef.current?.(cells);
    };

    const addCell = (x: number, y: number, alive: 0 | 1) => {
      stroke.set(`${x},${y}`, alive);
      if (!flushTimer) flushTimer = window.setTimeout(flushStroke, 40);
    };

    // Bresenham between consecutive pointer cells so fast drags leave no gaps.
    const paintLine = (
      from: { x: number; y: number },
      to: { x: number; y: number },
      alive: 0 | 1,
    ) => {
      let x = from.x;
      let y = from.y;
      const dx = Math.abs(to.x - x);
      const dy = -Math.abs(to.y - y);
      const sx = x < to.x ? 1 : -1;
      const sy = y < to.y ? 1 : -1;
      let err = dx + dy;
      for (;;) {
        addCell(x, y, alive);
        if (x === to.x && y === to.y) break;
        const e2 = 2 * err;
        if (e2 >= dy) {
          err += dy;
          x += sx;
        }
        if (e2 <= dx) {
          err += dx;
          y += sy;
        }
      }
    };

    const stampPattern = (e: PointerEvent) => {
      const f = frameRef.current;
      const p = patternRef.current;
      if (!f || !p) return;
      const anchor = screenToCell(e);
      if (!anchor) return;
      const ox = anchor.x - (p.w >> 1);
      const oy = anchor.y - (p.h >> 1);
      const cells: CellWrite[] = p.cells.map(([px, py]) => [
        (((ox + px) % f.width) + f.width) % f.width,
        (((oy + py) % f.height) + f.height) % f.height,
        1,
      ]);
      onPaintCellsRef.current?.(cells);
    };

    const onDown = (e: PointerEvent) => {
      if (toolRef.current === "draw" && e.button === 0) {
        if (patternRef.current) {
          // click stamps, drag pans
          stampDown = { x: e.clientX, y: e.clientY };
        } else {
          painting = true;
          erase = e.shiftKey;
          moved = false;
          downCell = screenToCell(e);
          lastCell = downCell;
          canvas.setPointerCapture(e.pointerId);
          return;
        }
      }
      dragging = true;
      lastX = e.clientX;
      lastY = e.clientY;
      canvas.setPointerCapture(e.pointerId);
    };
    const onMove = (e: PointerEvent) => {
      if (toolRef.current === "draw") {
        ghostCellRef.current = screenToCell(e);
      }
      if (painting) {
        const cell = screenToCell(e);
        if (!cell) {
          lastCell = null;
          return;
        }
        if (!moved && lastCell && cell.x === lastCell.x && cell.y === lastCell.y)
          return;
        moved = true;
        paintLine(lastCell ?? cell, cell, erase ? 0 : 1);
        lastCell = cell;
        return;
      }
      if (!dragging) return;
      const dx = (e.clientX - lastX) * dpr();
      const dy = (e.clientY - lastY) * dpr();
      lastX = e.clientX;
      lastY = e.clientY;
      const cam = cameraRef.current;
      cameraRef.current = {
        ...cam,
        originX: cam.originX + dx,
        originY: cam.originY + dy,
      };
      dirtyRef.current = true;
    };
    const onUp = (e: PointerEvent) => {
      if (painting) {
        painting = false;
        if (!moved && downCell) {
          addCell(downCell.x, downCell.y, cellAlive(downCell.x, downCell.y) ? 0 : 1);
        }
        flushStroke();
        downCell = lastCell = null;
      }
      if (stampDown && e.button === 0) {
        if (Math.hypot(e.clientX - stampDown.x, e.clientY - stampDown.y) < 4) {
          stampPattern(e);
        }
        stampDown = null;
      }
      dragging = false;
      try {
        canvas.releasePointerCapture(e.pointerId);
      } catch {}
    };
    const onLeave = () => {
      ghostCellRef.current = null;
    };
    const onContextMenu = (e: MouseEvent) => {
      if (toolRef.current === "draw") e.preventDefault();
    };
    const onWheel = (e: WheelEvent) => {
      e.preventDefault();
      const rect = canvas.getBoundingClientRect();
      const px = (e.clientX - rect.left) * dpr();
      const py = (e.clientY - rect.top) * dpr();
      const cam = cameraRef.current;
      const factor = Math.exp(-e.deltaY * 0.0015);
      const cell = Math.min(
        Math.max(cam.cellSize * factor, MIN_CELL),
        MAX_CELL,
      );
      const gx = (px - cam.originX) / cam.cellSize;
      const gy = (py - cam.originY) / cam.cellSize;
      cameraRef.current = {
        cellSize: cell,
        originX: px - gx * cell,
        originY: py - gy * cell,
      };
      dirtyRef.current = true;
    };

    canvas.addEventListener("pointerdown", onDown);
    canvas.addEventListener("pointermove", onMove);
    window.addEventListener("pointerup", onUp);
    canvas.addEventListener("wheel", onWheel, { passive: false });
    canvas.addEventListener("contextmenu", onContextMenu);
    canvas.addEventListener("pointerleave", onLeave);
    return () => {
      canvas.removeEventListener("pointerdown", onDown);
      canvas.removeEventListener("pointermove", onMove);
      window.removeEventListener("pointerup", onUp);
      canvas.removeEventListener("wheel", onWheel);
      canvas.removeEventListener("contextmenu", onContextMenu);
      canvas.removeEventListener("pointerleave", onLeave);
      if (flushTimer) window.clearTimeout(flushTimer);
    };
  }, []);

  return (
    <div className="stage">
      <canvas
        ref={canvasRef}
        className={"grid-canvas" + (tool === "draw" ? " draw" : "")}
      />
      <canvas ref={overlayRef} className="ghost-canvas" />
      {error && <div className="canvas-error">{error}</div>}
    </div>
  );
}

export const GridCanvas = memo(GridCanvasImpl);
