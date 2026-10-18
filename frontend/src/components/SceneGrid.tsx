import type { Scene } from "../types";

const GROUP_ORDER = ["White", "Functional", "Progressive", "Dynamic"];

export function SceneGrid({
  scenes,
  active,
  disabled,
  onPick,
}: {
  scenes: Scene[];
  active?: number;
  disabled: boolean;
  onPick: (id: number) => void;
}) {
  return (
    <div className="scenes">
      {GROUP_ORDER.map((group) => {
        const items = scenes.filter((s) => s.group === group);
        if (!items.length) return null;
        return (
          <div key={group} className="scene-group">
            <small>{group}</small>
            <div className="chips">
              {items.map((s) => (
                <button
                  key={s.id}
                  className={`chip ${active === s.id ? "active" : ""}`}
                  disabled={disabled}
                  onClick={() => onPick(s.id)}
                >
                  {s.name}
                </button>
              ))}
            </div>
          </div>
        );
      })}
    </div>
  );
}
