import { Bar, BarChart, CartesianGrid, ResponsiveContainer, Tooltip, XAxis, YAxis } from "recharts";
import type { CurrentMonth, MonthRow } from "../types";

export function MonthlyPanel({ current, history }: { current: CurrentMonth | null; history: MonthRow[] }) {
  if (!current) return <p className="muted">Loading…</p>;

  return (
    <div>
      <table className="slabs">
        <thead>
          <tr>
            <th>Slab</th>
            <th>Units</th>
            <th>Rate (₹)</th>
            <th>Amount (₹)</th>
          </tr>
        </thead>
        <tbody>
          {(current.slabs ?? []).map((l) => (
            <tr key={l.label}>
              <td>{l.label}</td>
              <td>{l.units.toFixed(2)}</td>
              <td>{l.rate.toFixed(2)}</td>
              <td>{l.amount.toFixed(2)}</td>
            </tr>
          ))}
          {(current.slabs ?? []).length === 0 && (
            <tr>
              <td colSpan={4} className="muted">No usage this month yet.</td>
            </tr>
          )}
          <tr className="total">
            <td>Total ({current.month})</td>
            <td>{current.kwh.toFixed(2)}</td>
            <td />
            <td>{current.bill.toFixed(2)}</td>
          </tr>
        </tbody>
      </table>

      <ResponsiveContainer width="100%" height={240}>
        <BarChart data={history} margin={{ top: 16, right: 16, left: 0, bottom: 0 }}>
          <CartesianGrid strokeDasharray="3 3" stroke="var(--grid)" />
          <XAxis dataKey="month" />
          <YAxis />
          <Tooltip formatter={(v) => `₹${Number(v).toFixed(2)}`} />
          <Bar dataKey="bill" name="Bill" fill="var(--accent)" radius={[6, 6, 0, 0]} />
        </BarChart>
      </ResponsiveContainer>

      <p className="muted small">
        Estimate for the devices SmartHub measures, using the slab rates. It excludes fixed charges,
        electricity duty and fuel adjustment, and it does not include the rest of your home.
      </p>
    </div>
  );
}
