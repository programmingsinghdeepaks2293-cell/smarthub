import type { CurrentMonth, EnergyRow, MonthRow, Range, Retention, SeriesDevice, Tip } from "./types";

async function get<T>(path: string): Promise<T> {
  const r = await fetch(`/api${path}`);
  if (!r.ok) throw new Error(await r.text());
  return r.json();
}

async function del(path: string): Promise<void> {
  const r = await fetch(`/api${path}`, { method: "DELETE" });
  if (!r.ok) throw new Error(await r.text());
}

export const api = {
  plugins: () => get<string[]>("/plugins"),
  series: (range: Range) => get<SeriesDevice[]>(`/telemetry/series?range=${range}`),
  energy: (range: Range) => get<EnergyRow[]>(`/energy/summary?range=${range}`),
  current: () => get<CurrentMonth>("/energy/current"),
  monthly: (months: number) => get<MonthRow[]>(`/energy/monthly?months=${months}`),
  retention: () => get<Retention>("/retention"),
  insights: () => get<Tip[]>("/insights"),
  deleteAll: () => del("/data?confirm=DELETE"),
  deleteDevice: (id: string) => del(`/data/devices/${encodeURIComponent(id)}?confirm=DELETE`),
  command: (id: string, action: string, value?: unknown) =>
    fetch(`/api/devices/${encodeURIComponent(id)}/command`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ action, value }),
    }).then(async (r) => {
      if (!r.ok) throw new Error(await r.text());
    }),
};
