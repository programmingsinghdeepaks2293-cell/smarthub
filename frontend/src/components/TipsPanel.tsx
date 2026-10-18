import type { Tip } from "../types";

const ICON: Record<Tip["severity"], string> = { warn: "⚠", tip: "💡", info: "ℹ" };

export function TipsPanel({ tips }: { tips: Tip[] | null }) {
  if (tips === null) return <p className="muted">Loading…</p>;
  if (tips.length === 0)
    return <p className="muted">No tips yet. SmartHub needs a few days of data to spot patterns.</p>;

  return (
    <ul className="tips">
      {tips.map((t, i) => (
        <li key={i} className={`tip ${t.severity}`}>
          <span className="tip-icon">{ICON[t.severity]}</span>
          <div>
            <strong>{t.title}</strong>
            <p>{t.detail}</p>
          </div>
        </li>
      ))}
    </ul>
  );
}
