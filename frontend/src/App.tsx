import { useEffect, useState } from "react";
import { api } from "./api";
import { useDevices } from "./useDevices";
import { DeviceCard } from "./components/DeviceCard";
import { PowerChart } from "./components/PowerChart";
import { Leaderboard } from "./components/Leaderboard";
import type { EnergyRow, Range, SeriesDevice } from "./types";
import "./App.css";

const RANGES: Range[] = ["1h", "24h", "7d"];

export default function App() {
  const devices = useDevices();
  const [range, setRange] = useState<Range>("24h");
  const [series, setSeries] = useState<SeriesDevice[]>([]);
  const [energy, setEnergy] = useState<EnergyRow[]>([]);
  const [plugins, setPlugins] = useState<string[]>([]);

  useEffect(() => {
    api.plugins().then(setPlugins).catch(console.error);
  }, []);

  useEffect(() => {
    const load = () => {
      api.series(range).then(setSeries).catch(console.error);
      api.energy(range).then(setEnergy).catch(console.error);
    };
    load();
    const t = setInterval(load, 15_000);
    return () => clearInterval(t);
  }, [range]);

  const liveW = devices.reduce((s, d) => s + (d.state.online ? d.state.power_w ?? 0 : 0), 0);
  const totalKWh = energy.reduce((s, r) => s + r.kwh, 0);
  const totalCost = energy.reduce((s, r) => s + r.cost, 0);
  const top = energy[0];

  return (
    <div className="app">
      <header className="topbar">
        <h1>SmartHub</h1>
        <div className="ranges">
          {RANGES.map((r) => (
            <button key={r} className={r === range ? "active" : ""} onClick={() => setRange(r)}>
              {r}
            </button>
          ))}
        </div>
      </header>

      <section className="kpis">
        <Kpi label="Live draw" value={`${liveW.toFixed(1)} W`} />
        <Kpi label={`Energy (${range})`} value={`${totalKWh.toFixed(3)} kWh`} />
        <Kpi label="Est. cost" value={totalCost.toFixed(2)} />
        <Kpi label="Top consumer" value={top ? top.name : "—"} sub={top ? `${top.kwh.toFixed(3)} kWh` : ""} />
      </section>

      <section className="grid">
        {devices.map((d) => (
          <DeviceCard key={d.id} d={d} />
        ))}
      </section>

      <section className="panel">
        <h2>Power over time</h2>
        <PowerChart series={series} />
      </section>

      <section className="panel">
        <h2>Who consumed the most</h2>
        <Leaderboard rows={energy} />
      </section>

      <footer className="muted">Loaded plugins: {plugins.join(", ") || "—"}</footer>
    </div>
  );
}

function Kpi({ label, value, sub }: { label: string; value: string; sub?: string }) {
  return (
    <div className="kpi">
      <small>{label}</small>
      <strong>{value}</strong>
      {sub && <small>{sub}</small>}
    </div>
  );
}
