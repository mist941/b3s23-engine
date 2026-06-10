import type { Status } from "./types";

const BASE = import.meta.env.VITE_ENGINE_URL ?? "";

export class EngineError extends Error {
  constructor(
    public readonly status: number,
    message: string,
  ) {
    super(message);
    this.name = "EngineError";
  }
}

async function req<T>(
  method: string,
  path: string,
  body?: unknown,
): Promise<T> {
  const hasBody = body !== undefined;
  const res = await fetch(`${BASE}/api/v1${path}`, {
    method,
    headers: hasBody ? { "Content-Type": "application/json" } : undefined,
    body: hasBody ? JSON.stringify(body) : undefined,
  });
  if (!res.ok) {
    let msg = `${res.status} ${res.statusText}`;
    try {
      const data = (await res.json()) as { error?: string };
      if (data?.error) msg = data.error;
    } catch {}
    throw new EngineError(res.status, msg);
  }
  if ((res.headers.get("content-type") ?? "").includes("application/json")) {
    return (await res.json()) as T;
  }
  return undefined as T;
}

export const api = {
  getStatus: () => req<Status>("GET", "/status"),
  play: () => req<Status>("POST", "/play"),
  pause: () => req<Status>("POST", "/pause"),
  step: () => req<Status>("POST", "/step"),
  reset: (rngSeed?: number) =>
    req<Status>("POST", "/reset", rngSeed !== undefined ? { rngSeed } : {}),
  reseed: (probability?: number, rngSeed?: number) =>
    req<Status>("POST", "/reseed", { probability, rngSeed }),
  setProbability: (probability: number, reseed: boolean) =>
    req<Status>("PUT", "/config/probability", { probability, reseed }),
  setSize: (width: number, height: number) =>
    req<Status>("PUT", "/config/size", { width, height }),
  setTickRate: (tickHz: number) =>
    req<Status>("PUT", "/config/tickrate", { tickHz }),
  setStreamRate: (streamEveryN: number) =>
    req<Status>("PUT", "/config/streamrate", { streamEveryN }),
  snapshot: () => req<{ snapshotGeneration?: number }>("POST", "/snapshot"),
};
