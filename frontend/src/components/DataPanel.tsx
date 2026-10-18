import { useState } from "react";
import { api } from "../api";
import type { Device, Retention } from "../types";

type Target = { kind: "all" } | { kind: "device"; id: string; name: string };

export function DataPanel({
  devices,
  retention,
  onDeleted,
}: {
  devices: Device[];
  retention: Retention | null;
  onDeleted: () => void;
}) {
  const [target, setTarget] = useState<Target | null>(null);
  const [typed, setTyped] = useState("");
  const [busy, setBusy] = useState(false);
  const [msg, setMsg] = useState("");

  const cancel = () => {
    setTarget(null);
    setTyped("");
  };

  const confirm = async () => {
    if (!target || typed !== "DELETE") return;
    setBusy(true);
    setMsg("");
    try {
      if (target.kind === "all") {
        await api.deleteAll();
        setMsg("All energy data deleted.");
      } else {
        await api.deleteDevice(target.id);
        setMsg(`Deleted history for ${target.name}.`);
      }
      onDeleted();
    } catch (e) {
      setMsg(String(e));
    } finally {
      setBusy(false);
      cancel();
    }
  };

  return (
    <div className="data">
      {retention && (
        <p className="muted small">
          Live samples are kept for {retention.raw_days} days. Hourly energy totals, used for bills and
          history, are kept for {retention.hourly_years} years. Older data is removed automatically.
        </p>
      )}

      <div className="data-actions">
        <button className="danger" onClick={() => setTarget({ kind: "all" })} disabled={busy}>
          Delete all energy data
        </button>
      </div>

      <ul className="device-data">
        {devices.map((d) => (
          <li key={d.id}>
            <span>{d.name}</span>
            <button
              className="ghost"
              onClick={() => setTarget({ kind: "device", id: d.id, name: d.name })}
              disabled={busy}
            >
              Delete history
            </button>
          </li>
        ))}
      </ul>

      {target && (
        <div className="confirm-box">
          <p>
            {target.kind === "all"
              ? "This permanently deletes all recorded energy and power history for every device. Device settings are kept."
              : `This permanently deletes all recorded history for ${target.name}.`}
          </p>
          <p className="small">Type <strong>DELETE</strong> to confirm.</p>
          <input
            value={typed}
            onChange={(e) => setTyped(e.target.value)}
            placeholder="DELETE"
            autoFocus
          />
          <div className="data-actions">
            <button className="danger" disabled={typed !== "DELETE" || busy} onClick={confirm}>
              Confirm delete
            </button>
            <button className="ghost" onClick={cancel}>Cancel</button>
          </div>
        </div>
      )}

      {msg && <p className="small">{msg}</p>}
    </div>
  );
}
