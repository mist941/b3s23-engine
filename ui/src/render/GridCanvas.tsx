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

export interface GridColors {
  alive: string;
  dead: string;
  bg: string;
  grid: string;
}

export interface GridHandle {
  pushFrame: (frame: GridFrame) => void;
}

interface Props {
  colors: GridColors;
  gridLines?: boolean;
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

function GridCanvasImpl({ colors, gridLines = true, onFps, ref }: Props) {
  const canvasRef = useRef<HTMLCanvasElement | null>(null);
  const frameRef = useRef<GridFrame | null>(null);
  const uploadPendingRef = useRef(false);
  const cameraRef = useRef<Camera>({ cellSize: 1, originX: 0, originY: 0 });
  const dirtyRef = useRef(true);
  const fittedDimsRef = useRef<{ w: number; h: number } | null>(null);
  const gridLinesRef = useRef(gridLines);
  gridLinesRef.current = gridLines;

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

    const onDown = (e: PointerEvent) => {
      dragging = true;
      lastX = e.clientX;
      lastY = e.clientY;
      canvas.setPointerCapture(e.pointerId);
    };
    const onMove = (e: PointerEvent) => {
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
      dragging = false;
      try {
        canvas.releasePointerCapture(e.pointerId);
      } catch {}
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
    return () => {
      canvas.removeEventListener("pointerdown", onDown);
      canvas.removeEventListener("pointermove", onMove);
      window.removeEventListener("pointerup", onUp);
      canvas.removeEventListener("wheel", onWheel);
    };
  }, []);

  return (
    <div className="stage">
      <canvas ref={canvasRef} className="grid-canvas" />
      {error && <div className="canvas-error">{error}</div>}
    </div>
  );
}

export const GridCanvas = memo(GridCanvasImpl);
