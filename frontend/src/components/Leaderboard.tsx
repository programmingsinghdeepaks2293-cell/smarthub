import { Bar, BarChart, Cell, ResponsiveContainer, Tooltip, XAxis, YAxis } from "recharts";
import type { EnergyRow } from "../types";
import { colorFor } from "../colors";

export function Leaderboard({ rows }: { rows: EnergyRow[] }) {
  if (!rows.length) return <p className="muted">No energy data yet. Give it a few minutes.</p>;

  return (
    <div>
      <ResponsiveContainer width="100%" height={Math.max(160, rows.length * 56)}>
        <BarChart data={rows} layout="vertical" margin={{ left: 10, right: 30 }}>
          <XAxis type="number" unit=" kWh" />
          <YAxis type="category" dataKey="name" width={160} />
          <Tooltip formatter={(v) => `${Number(v).toFixed(3)} kWh`} />
          <Bar dataKey="kwh" radius={[0, 8, 8, 0]}>
            {rows.map((r, i) => (
              <Cell key={r.device_id} fill={colorFor(i)} />
            ))}
          </Bar>
        </BarChart>
      </ResponsiveContainer>

      <ol className="ranked">
        {rows.map((r, i) => (
          <li key={r.device_id}>
            <span>#{i + 1} {r.name}</span>
            <span>
              {r.avg_w.toFixed(1)} W avg{r.estimated && " (est.)"} · {r.cost.toFixed(2)} cost
            </span>
          </li>
        ))}
      </ol>
    </div>
  );
}
