"use client";

import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { useEffect, useRef, useState, useCallback } from "react";
import { metricsApi, type SystemInfoResponse, type ProcessListResponse } from "@/lib/api/metrics";
import { tokenStore } from "@/lib/api/auth";

// ── Snapshot (Polling Fallback) ───────────────────────────────────────────────

export function useMetricsSnapshot() {
  return useQuery({
    queryKey: ["metrics", "snapshot"],
    queryFn: () => metricsApi.getSnapshot(),
    refetchInterval: 5_000,
    retry: 1,
  });
}

// ── WebSocket Live-Stream ─────────────────────────────────────────────────────

export interface MetricsSnapshot {
  timestamp: string;
  cpu: {
    usage_percent: number;
    core_usage_percent: number[];
    num_cores: number;
  };
  ram: {
    total_bytes: number;
    used_bytes: number;
    free_bytes: number;
    available_bytes: number;
    cached_bytes: number;
    swap_total_bytes: number;
    swap_used_bytes: number;
    usage_percent: number;
  };
  disks: Array<{
    device: string;
    mount_point: string;
    fs_type: string;
    total_bytes: number;
    used_bytes: number;
    usage_percent: number;
    read_bytes_per_sec: number;
    write_bytes_per_sec: number;
  }>;
  networks: Array<{
    interface: string;
    type: string;
    link_state: string;
    rx_bytes_per_sec: number;
    tx_bytes_per_sec: number;
  }>;
  load_avg: {
    load_1: number;
    load_5: number;
    load_15: number;
  };
  uptime_sec: number;
}

export function useMetricsWebSocket() {
  const [latest, setLatest] = useState<MetricsSnapshot | null>(null);
  const [history, setHistory] = useState<MetricsSnapshot[]>([]);
  const [connected, setConnected] = useState(false);
  const wsRef = useRef<WebSocket | null>(null);
  const reconnectRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const mountedRef = useRef(true);

  const connect = useCallback(() => {
    const token = tokenStore.getAccess();
    if (!token || !mountedRef.current) return;

    const protocol = window.location.protocol === "https:" ? "wss:" : "ws:";
    const host = process.env.NEXT_PUBLIC_WS_HOST ?? window.location.host;
    const url = `${protocol}//${host}/ws/metrics?token=${token}`;

    const ws = new WebSocket(url);
    wsRef.current = ws;

    ws.onopen = () => {
      if (!mountedRef.current) return;
      setConnected(true);
    };

    ws.onmessage = (event) => {
      if (!mountedRef.current) return;
      try {
        const msg = JSON.parse(event.data) as { type: string; data: unknown };

        if (msg.type === "metrics") {
          const snap = msg.data as MetricsSnapshot;
          setLatest(snap);
          setHistory((prev) => {
            const next = [...prev, snap];
            return next.slice(-60); // max 60s
          });
        }

        if (msg.type === "history") {
          const snaps = msg.data as MetricsSnapshot[];
          setHistory(snaps.slice(-60));
          if (snaps.length > 0) {
            setLatest(snaps[snaps.length - 1]);
          }
        }
      } catch {
        // Ungültiges JSON — ignorieren
      }
    };

    ws.onclose = () => {
      if (!mountedRef.current) return;
      setConnected(false);
      // Reconnect nach 3s
      reconnectRef.current = setTimeout(() => {
        if (mountedRef.current) connect();
      }, 3_000);
    };

    ws.onerror = () => {
      ws.close();
    };
  }, []);

  useEffect(() => {
    mountedRef.current = true;
    connect();

    return () => {
      mountedRef.current = false;
      if (reconnectRef.current) clearTimeout(reconnectRef.current);
      if (wsRef.current) wsRef.current.close();
    };
  }, [connect]);

  return { latest, history, connected };
}

// ── System Info ───────────────────────────────────────────────────────────────

export function useSystemInfo() {
  return useQuery<SystemInfoResponse>({
    queryKey: ["system", "info"],
    queryFn: () => metricsApi.getSystemInfo(),
    staleTime: 30_000,
    retry: 1,
  });
}

// ── Prozesse ──────────────────────────────────────────────────────────────────

export function useProcesses(sort = "mem") {
  return useQuery<ProcessListResponse>({
    queryKey: ["processes", sort],
    queryFn: () => metricsApi.getProcesses(sort),
    refetchInterval: 10_000,
    retry: 1,
  });
}

export function useKillProcess() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (pid: number) => metricsApi.killProcess(pid),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["processes"] });
    },
  });
}

// ── Hilfsfunktionen ───────────────────────────────────────────────────────────

export function formatBytes(bytes: number, decimals = 1): string {
  if (bytes === 0) return "0 B";
  const k = 1024;
  const sizes = ["B", "KB", "MB", "GB", "TB"];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return `${parseFloat((bytes / Math.pow(k, i)).toFixed(decimals))} ${sizes[i]}`;
}

export function formatBytesPerSec(bps: number): string {
  return `${formatBytes(bps)}/s`;
}

export function formatUptime(seconds: number): string {
  const d = Math.floor(seconds / 86400);
  const h = Math.floor((seconds % 86400) / 3600);
  const m = Math.floor((seconds % 3600) / 60);
  if (d > 0) return `${d}d ${h}h ${m}m`;
  if (h > 0) return `${h}h ${m}m`;
  return `${m}m`;
}
