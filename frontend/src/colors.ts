const PALETTE = ["#60a5fa", "#34d399", "#f472b6", "#fbbf24", "#a78bfa", "#f87171"];
export const colorFor = (i: number) => PALETTE[i % PALETTE.length];
