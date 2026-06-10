import type { Status } from "../api/types";

interface Props {
  status: Status | null;
  connected: boolean;
  fps: number;
  live: { generation: number; population: number } | null;
}

function fmtInt(n: number): string {
  return n.toLocaleString("en-US");
}

function fmtTime(iso: string): string {
  const d = new Date(iso);
  return Number.isNaN(d.getTime()) ? "—" : d.toLocaleString();
}

export function StatusBar({ status, connected, fps, live }: Props) {
  const generation = live?.generation ?? status?.generation ?? 0;
  const population = live?.population ?? status?.population ?? 0;
  const state = status?.state ?? "—";

  return (
    <div className="statusbar">
      <span className={`conn ${connected ? "on" : "off"}`}>
        <span className="dot" />
        {connected ? "connected" : "disconnected"}
      </span>
      <span className={`badge state-${state}`}>{state}</span>
      <Stat label="generation" value={fmtInt(generation)} />
      <Stat label="population" value={fmtInt(population)} />
      <Stat label="epoch" value={status ? String(status.epoch) : "—"} />
      <Stat label="size" value={status ? `${status.width}×${status.height}` : "—"} />
      <Stat label="tick" value={status ? `${status.tickHz} Hz` : "—"} />
      <Stat label="stream" value={status ? `1/${status.streamEveryN}` : "—"} />
      <Stat label="started" value={status ? fmtTime(status.startTime) : "—"} />
      <Stat label="render" value={`${fps.toFixed(0)} fps`} />
    </div>
  );
}

function Stat({ label, value }: { label: string; value: string }) {
  return (
    <span className="stat">
      <span className="stat-label">{label}</span>
      <span className="stat-value">{value}</span>
    </span>
  );
}
