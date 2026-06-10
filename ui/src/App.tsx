import { useEffect, useRef, useState } from "react";

import { api } from "./api/client";
import type { Status } from "./api/types";
import { useEngineSocket } from "./ws/useEngineSocket";
import { GridCanvas, type GridHandle } from "./render/GridCanvas";
import { StatusBar } from "./components/StatusBar";
import { Controls } from "./components/Controls";

const COLORS = {
  alive: "#39ff14",
  dead: "#0c1116",
  bg: "#05080a",
  grid: "#1d2935",
};

export default function App() {
  const [status, setStatus] = useState<Status | null>(null);
  const [fps, setFps] = useState(0);
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const [live, setLive] = useState<{
    generation: number;
    population: number;
  } | null>(null);
  const canvasRef = useRef<GridHandle>(null);

  const { connected } = useEngineSocket({
    onStatus: (s) => {
      setStatus(s);
      setLive({ generation: s.generation, population: s.population });
    },
    onFrame: (frame, meta) => {
      canvasRef.current?.pushFrame(frame);
      if (meta)
        setLive({ generation: meta.generation, population: meta.population });
    },
  });

  useEffect(() => {
    api
      .getStatus()
      .then(setStatus)
      .catch(() => undefined);
  }, []);

  return (
    <div className="app">
      <header className="topbar">
        <h1>
          B3S23 <span className="muted">Engine</span>
        </h1>
        <StatusBar
          status={status}
          connected={connected}
          fps={fps}
          live={live}
        />
      </header>

      <main className="stage-wrap">
        <GridCanvas ref={canvasRef} colors={COLORS} onFps={setFps} />
        <div className="overlay-hint">drag to pan · scroll to zoom</div>
      </main>

      <aside className="panel">
        <Controls
          status={status}
          onError={setError}
          busy={busy}
          setBusy={setBusy}
        />
        {error && <div className="error-banner">⚠ {error}</div>}
      </aside>
    </div>
  );
}
