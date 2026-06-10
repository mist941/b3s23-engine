import { useEffect, useRef, useState } from "react";

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

interface Options {
  onStatus: (s: Status) => void;
  onFrame: (frame: GridFrame, meta: FrameMeta | null) => void;
}

export function useEngineSocket(opts: Options): { connected: boolean } {
  const [connected, setConnected] = useState(false);
  const optsRef = useRef(opts);
  optsRef.current = opts;

  useEffect(() => {
    let ws: WebSocket | null = null;
    let stopped = false;
    let retry = 0;
    let reconnectTimer: number | undefined;
    let pendingMeta: FrameMeta | null = null;

    const connect = () => {
      ws = new WebSocket(wsUrl());
      ws.binaryType = "arraybuffer";

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
    };
  }, []);

  return { connected };
}
