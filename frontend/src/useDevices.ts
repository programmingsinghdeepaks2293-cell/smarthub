import { useEffect, useState } from "react";
import type { Device } from "./types";

export function useDevices(): Device[] {
  const [devices, setDevices] = useState<Device[]>([]);

  useEffect(() => {
    const es = new EventSource("/api/events");
    es.addEventListener("state", (e: MessageEvent) => {
      const d: Device = JSON.parse(e.data);
      setDevices((prev) => {
        const i = prev.findIndex((x) => x.id === d.id);
        if (i === -1) return [...prev, d];
        const next = [...prev];
        next[i] = d;
        return next;
      });
    });
    return () => es.close();
  }, []);

  return devices;
}
