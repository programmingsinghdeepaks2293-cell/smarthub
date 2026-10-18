import { useState } from "react";
import { api } from "../api";
import type { Capability, Device } from "../types";
import { SceneGrid } from "./SceneGrid";

const hexToRgb = (hex: string) => ({
  r: parseInt(hex.slice(1, 3), 16),
  g: parseInt(hex.slice(3, 5), 16),
  b: parseInt(hex.slice(5, 7), 16),
});

export function DeviceCard({ d }: { d: Device }) {
  const { state } = d;
  const [bri, setBri] = useState<number | null>(null);
  const [kelvin, setKelvin] = useState<number | null>(null);
  const [color, setColor] = useState<string>("#ffffff");
  const [busy, setBusy] = useState(false);
  const has = (c: Capability) => d.capabilities.includes(c);

  const send = async (action: string, value?: unknown) => {
    setBusy(true);
    try {
      await api.command(d.id, action, value);
    } catch (e) {
      alert(String(e));
    } finally {
      setBusy(false);
    }
  };

  return (
    <article className={`card ${state.on ? "on" : ""} ${state.online ? "" : "offline"}`}>
      <header>
        <div>
          <h3>{d.name}</h3>
          <small>{d.vendor} · {d.transport}</small>
        </div>
        {has("switch") && (
          <button
            className="switch"
            aria-pressed={state.on}
            disabled={!state.online || busy}
            onClick={() => send("toggle")}
          >
            <span />
          </button>
        )}
      </header>

      {has("power_meter") && (
        <div className="watts">
          {state.power_w?.toFixed(1) ?? "—"}<small> W</small>
          {state.power_estimated && <span className="tag">est.</span>}
        </div>
      )}

      {has("brightness") && (
        <input
          type="range"
          min={10}
          max={100}
          value={bri ?? state.brightness ?? 0}
          disabled={!state.online || !state.on}
          onChange={(e) => setBri(Number(e.target.value))}
          onPointerUp={() => {
            if (bri !== null) {
              send("brightness", bri);
              setBri(null);
            }
          }}
        />
      )}

      {(has("color") || has("temperature")) && (
        <div className="ctrl-row">
          {has("color") && (
            <label className="swatch" title="Color">
              <input
                type="color"
                value={color}
                disabled={!state.online}
                onChange={(e) => setColor(e.target.value)}
                onBlur={() => send("color", hexToRgb(color))}
              />
              <span style={{ background: color }} />
            </label>
          )}
          {has("temperature") && (
            <div className="temp">
              <input
                type="range"
                min={2200}
                max={6500}
                step={100}
                value={kelvin ?? 2700}
                disabled={!state.online}
                onChange={(e) => setKelvin(Number(e.target.value))}
                onPointerUp={() => {
                  if (kelvin !== null) {
                    send("temp", kelvin);
                    setKelvin(null);
                  }
                }}
              />
              <small>{kelvin ?? 2700} K</small>
            </div>
          )}
        </div>
      )}

      {has("scenes") && d.scenes && (
        <details>
          <summary>Scenes</summary>
          <SceneGrid
            scenes={d.scenes}
            active={state.scene}
            disabled={!state.online}
            onPick={(id) => send("scene", id)}
          />
        </details>
      )}

      {!state.online && <p className="err">{state.error ?? "offline"}</p>}
    </article>
  );
}
