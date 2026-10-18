import { useMemo } from "react";
import { Area, AreaChart, CartesianGrid, Legend, ResponsiveContainer, Tooltip, XAxis, YAxis } from "recharts";
import type { SeriesDevice } from "../types";
import { colorFor } from "../colors";

export function PowerChart({ series }: { series: SeriesDevice[] }) {
  const rows = useMemo(() => {
    const byT = new Map<number, Record<string, number>>();
    for (const s of series) {
      for (const p of s.points) {
        const row = byT.get(p.t) ?? { t: p.t };
        row[s.device_id] = Math.round(p.w * 10) / 10;
        byT.set(p.t, row);
      }
    }
    return [...byT.values()].sort((a, b) => a.t - b.t);
  }, [series]);

  return (
    <ResponsiveContainer width="100%" height={320}>
      <AreaChart data={rows}>
        <defs>
          {series.map((s, i) => (
            <linearGradient key={s.device_id} id={`g-${s.device_id}`} x1="0" y1="0" x2="0" y2="1">
              <stop offset="0%" stopColor={colorFor(i)} stopOpacity={0.5} />
              <stop offset="100%" stopColor={colorFor(i)} stopOpacity={0} />
            </linearGradient>
          ))}
        </defs>
        <CartesianGrid strokeDasharray="3 3" stroke="var(--grid)" />
        <XAxis
          dataKey="t"
          tickFormatter={(t) => new Date(Number(t) * 1000).toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" })}
        />
        <YAxis unit=" W" />
        <Tooltip labelFormatter={(t) => new Date(Number(t) * 1000).toLocaleString()} />
        <Legend />
        {series.map((s, i) => (
          <Area
            key={s.device_id}
            type="monotone"
            dataKey={s.device_id}
            name={s.name}
            stroke={colorFor(i)}
            fill={`url(#g-${s.device_id})`}
            connectNulls
          />
        ))}
      </AreaChart>
    </ResponsiveContainer>
  );
}
