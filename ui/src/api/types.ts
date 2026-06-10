export type RunState = "running" | "paused" | "stopped";

export interface Status {
  state: RunState;
  generation: number;
  epoch: number;
  width: number;
  height: number;
  probability: number;
  tickHz: number;
  streamEveryN: number;
  population: number;
  startTime: string;
}
