import type { EnergyRow, Range, SeriesDevice } from "./types";

async function get<T>(path: string): Promise<T> {
  const r = await fetch(`/api${path}`);
  if (!r.ok) throw new Error(await r.text());
  return r.json();
}

export const api = {
  plugins: () => get<string[]>("/plugins"),
  series: (range: Range) => get<SeriesDevice[]>(`/telemetry/series?range=${range}`),
  energy: (range: Range) => get<EnergyRow[]>(`/energy/summary?range=${range}`),
  command: (id: string, action: string, value?: unknown) =>
    fetch(`/api/devices/${encodeURIComponent(id)}/command`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ action, value }),
    }).then(async (r) => {
      if (!r.ok) throw new Error(await r.text());
    }),
};
