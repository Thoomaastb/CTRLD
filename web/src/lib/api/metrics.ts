import { apiClient } from "./client";
import type { SystemMetrics, SystemInfo } from "@/types/api";

export { type SystemMetrics, type SystemInfo };

// ── API Calls ─────────────────────────────────────────────────────────────────

export const metricsApi = {
  getSnapshot: () => apiClient.get<SystemMetrics>("/system/metrics"),
  getHistory: () => apiClient.get<SystemMetrics[]>("/system/metrics/history"),
  getSystemInfo: () => apiClient.get<SystemInfoResponse>("/system/info"),
  getProcesses: (sort = "mem", limit = 50) =>
    apiClient.get<ProcessListResponse>(`/system/processes?sort=${sort}&limit=${limit}`),
  killProcess: (pid: number) =>
    apiClient.delete<void>(`/system/processes/${pid}`),
};

export interface SystemInfoResponse {
  system: {
    hostname: string;
    os: string;
    kernel_version: string;
    architecture: string;
    cpu_model: string;
    cpu_cores: number;
    ram_total_bytes: number;
    docker?: {
      available: boolean;
      version?: string;
      endpoint: string;
      containers?: DockerContainer[];
    };
    collected_at: string;
  };
  networks?: NetworkInterface[];
  disk?: DiskInfo[];
}

export interface DockerContainer {
  id: string;
  name: string;
  image: string;
  state: string;
  status: string;
  ports?: string[];
}

export interface NetworkInterface {
  interface: string;
  ip_addresses: string[];
  mac_address: string;
  type: string;
  link_state: string;
  rx_bytes_per_sec: number;
  tx_bytes_per_sec: number;
  rx_bytes_total: number;
  tx_bytes_total: number;
}

export interface DiskInfo {
  device: string;
  mount_point: string;
  fs_type: string;
  total_bytes: number;
  used_bytes: number;
  free_bytes: number;
  usage_percent: number;
  read_bytes_per_sec: number;
  write_bytes_per_sec: number;
}

export interface ProcessInfo {
  pid: number;
  name: string;
  cpu_percent: number;
  mem_percent: number;
  mem_bytes: number;
  status: string;
}

export interface ProcessListResponse {
  processes: ProcessInfo[];
  total: number;
}
