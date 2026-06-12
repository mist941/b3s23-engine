import { useEffect, useRef, useState } from "react";

import { api } from "../api/client";
import type { Status } from "../api/types";
import type { Tool } from "../render/GridCanvas";
import { PATTERNS } from "../patterns";

interface Props {
  status: Status | null;
  onError: (msg: string | null) => void;
  busy: boolean;
  setBusy: (b: boolean) => void;
  tool: Tool;
  setTool: (t: Tool) => void;
  patternId: string;
  setPatternId: (id: string) => void;
}

export function Controls({
  status,
  onError,
  busy,
  setBusy,
  tool,
  setTool,
  patternId,
  setPatternId,
}: Props) {
  const running = status?.state === "running";

  const [prob, setProb] = useState(0.1);
  const [tickHz, setTickHz] = useState(5);
  const [width, setWidth] = useState(1024);
  const [height, setHeight] = useState(1024);
  const seeded = useRef(false);

  useEffect(() => {
    if (status && !seeded.current) {
      seeded.current = true;
      setProb(status.probability);
      setTickHz(status.tickHz);
      setWidth(status.width);
      setHeight(status.height);
    }
  }, [status]);

  const run = async (fn: () => Promise<unknown>) => {
    onError(null);
    setBusy(true);
    try {
      await fn();
    } catch (e) {
      onError(e instanceof Error ? e.message : String(e));
    } finally {
      setBusy(false);
    }
  };

  const applySize = () =>
    run(async () => {
      if (running) await api.pause(); // resize is rejected while running
      await api.setSize(width, height);
    });

  return (
    <div className="controls">
      <section>
        <h3>Tool</h3>
        <div className="row seg">
          <button
            className={tool === "pan" ? "active" : ""}
            onClick={() => setTool("pan")}
          >
            ✋ Pan
          </button>
          <button
            className={tool === "draw" ? "active" : ""}
            onClick={() => setTool("draw")}
          >
            ✏️ Draw
          </button>
        </div>
        <label className="field">
          <span>Pattern</span>
          <select
            value={patternId}
            onChange={(e) => {
              setPatternId(e.target.value);
              if (e.target.value) setTool("draw");
            }}
          >
            <option value="">Freehand</option>
            {PATTERNS.map((p) => (
              <option key={p.id} value={p.id}>
                {p.name}
              </option>
            ))}
          </select>
        </label>
      </section>

      <section>
        <h3>Loop</h3>
        <div className="row">
          {running ? (
            <button onClick={() => run(api.pause)} disabled={busy}>
              ⏸ Pause
            </button>
          ) : (
            <button
              className="primary"
              onClick={() => run(api.play)}
              disabled={busy}
            >
              ▶ Play
            </button>
          )}
          <button
            onClick={() => run(api.step)}
            disabled={busy || running}
            title="Advance one generation (paused only)"
          >
            ⏭ Step
          </button>
        </div>
        <div className="row">
          <button onClick={() => run(() => api.reset())} disabled={busy}>
            ↺ Reset
          </button>
        </div>
      </section>

      <section>
        <h3>Seed</h3>
        <label className="field">
          <span>
            Probability <b>{prob.toFixed(2)}</b>
          </span>
          <input
            type="range"
            min={0}
            max={1}
            step={0.01}
            value={prob}
            onChange={(e) => setProb(Number(e.target.value))}
          />
        </label>
        <div className="row">
          <button
            className="primary"
            onClick={() => run(() => api.reseed(prob))}
            disabled={busy}
            title="New epoch, generation 0, reseed at this probability"
          >
            🎲 Reseed
          </button>
        </div>
      </section>

      <section>
        <h3>Rates</h3>
        <label className="field">
          <span>Tick rate (Hz)</span>
          <div className="row">
            <input
              type="number"
              min={0.1}
              step={0.5}
              value={tickHz}
              onChange={(e) => setTickHz(Number(e.target.value))}
            />
            <button
              onClick={() => run(() => api.setTickRate(tickHz))}
              disabled={busy}
            >
              Apply
            </button>
          </div>
        </label>
      </section>

      <section>
        <h3>Size</h3>
        <div className="row">
          <label className="field grow">
            <span>Width</span>
            <input
              type="number"
              min={8}
              step={64}
              value={width}
              onChange={(e) => setWidth(Number(e.target.value))}
            />
          </label>
          <label className="field grow">
            <span>Height</span>
            <input
              type="number"
              min={8}
              step={64}
              value={height}
              onChange={(e) => setHeight(Number(e.target.value))}
            />
          </label>
        </div>
        <div className="row">
          <button
            onClick={applySize}
            disabled={busy}
            title="Pauses first if running, then resizes + reseeds"
          >
            Apply size
          </button>
        </div>
        <p className="hint">
          Resizing pauses the loop, starts a new epoch and reseeds.
        </p>
      </section>
    </div>
  );
}
