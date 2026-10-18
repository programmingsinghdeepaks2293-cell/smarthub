export type Capability = "switch" | "brightness" | "power_meter" | "scenes" | "color" | "temperature";
export type Range = "1h" | "24h" | "7d";

export interface Scene {
  id: number;
  name: string;
  group: string;
}

export interface Device {
  id: string;
  name: string;
  kind: string;
  vendor: string;
  transport: string;
  capabilities: Capability[];
  scenes?: Scene[];
  state: {
    online: boolean;
    on: boolean;
    brightness?: number;
    power_w?: number;
    power_estimated?: boolean;
    scene?: number;
    error?: string;
    updated_at: string;
  };
}

export interface SeriesDevice {
  device_id: string;
  name: string;
  points: { t: number; w: number }[];
}

export interface EnergyRow {
  device_id: string;
  name: string;
  kwh: number;
  avg_w: number;
  cost: number;
  estimated: boolean;
}

export interface SlabLine {
  label: string;
  units: number;
  rate: number;
  amount: number;
}

export interface MonthDevice {
  device_id: string;
  name: string;
  kwh: number;
  cost: number;
}

export interface CurrentMonth {
  month: string;
  kwh: number;
  bill: number;
  slabs: SlabLine[];
  devices: MonthDevice[];
}

export interface MonthRow {
  month: string;
  kwh: number;
  bill: number;
  devices: MonthDevice[];
}

export interface Retention {
  raw_days: number;
  hourly_years: number;
}
