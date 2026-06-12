import { useCallback, useEffect, useRef, useState } from "react";

import type { Status } from "../api/types";
import { decodeFrame, type GridFrame } from "../grid/decode";

const BASE = import.meta.env.VITE_ENGINE_URL ?? "";

function wsUrl(): string {
  if (BASE) {
    return BASE.replace(/^http/, "ws") + "/api/v1/ws";
  }
  const proto = location.protocol === "https:" ? "wss" : "ws";
  return `${proto}://${location.host}/api/v1/ws`;
}

export interface FrameMeta {
  generation: number;
  epoch: number;
  population: number;
}

/** [x, y, alive] triplet, alive is 0 or 1 — mirrors the engine wire format. */
export type CellWrite = [number, number, 0 | 1];

const MAX_CELL_BATCH = 4096;

interface Options {
  onStatus: (s: Status) => void;
  onFrame: (frame: GridFrame, meta: FrameMeta | null) => void;
}

export function useEngineSocket(opts: Options): {
  connected: boolean;
  sendCells: (cells: CellWrite[]) => void;
} {
  const [connected, setConnected] = useState(false);
  const optsRef = useRef(opts);
  optsRef.current = opts;
  const wsRef = useRef<WebSocket | null>(null);

  const sendCells = useCallback((cells: CellWrite[]) => {
    const ws = wsRef.current;
    if (!ws || ws.readyState !== WebSocket.OPEN || cells.length === 0) return;
    for (let i = 0; i < cells.length; i += MAX_CELL_BATCH) {
      ws.send(
        JSON.stringify({
          type: "setCells",
          cells: cells.slice(i, i + MAX_CELL_BATCH),
        }),
      );
    }
  }, []);

  useEffect(() => {
    let ws: WebSocket | null = null;
    let stopped = false;
    let retry = 0;
    let reconnectTimer: number | undefined;
    let pendingMeta: FrameMeta | null = null;

    const connect = () => {
      ws = new WebSocket(wsUrl());
      ws.binaryType = "arraybuffer";
      wsRef.current = ws;

      ws.onopen = () => {
        retry = 0;
        setConnected(true);
      };

      ws.onclose = () => {
        setConnected(false);
        if (!stopped) {
          const delay = Math.min(500 * 2 ** retry, 10000);
          retry++;
          reconnectTimer = window.setTimeout(connect, delay);
        }
      };

      ws.onerror = () => ws?.close();

      ws.onmessage = (ev) => {
        if (typeof ev.data === "string") {
          try {
            const msg = JSON.parse(ev.data) as { type?: string } & Record<
              string,
              unknown
            >;
            if (msg.type === "status") {
              optsRef.current.onStatus(msg as unknown as Status);
            } else if (msg.type === "gen") {
              pendingMeta = {
                generation: msg.generation as number,
                epoch: msg.epoch as number,
                population: msg.population as number,
              };
            }
          } catch {}
          return;
        }
        try {
          const frame = decodeFrame(ev.data as ArrayBuffer);
          optsRef.current.onFrame(frame, pendingMeta);
          pendingMeta = null;
        } catch (e) {
          console.error("failed to decode grid frame", e);
        }
      };
    };

    connect();

    return () => {
      stopped = true;
      if (reconnectTimer) window.clearTimeout(reconnectTimer);
      if (ws) {
        ws.onclose = null;
        ws.close();
      }
      wsRef.current = null;
    };
  }, []);

  return { connected, sendCells };
}
