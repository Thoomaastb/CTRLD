"use client";

import type { Metadata } from "next";
import { Cpu, HardDrive, MemoryStick, Network, Timer, Server } from "lucide-react";
import { MetricCard } from "@/components/dashboard/metric-card";
import { useMetricsWebSocket, formatBytes, formatBytesPerSec, formatUptime } from "@/lib/hooks/use-metrics";
import { useSystemInfo } from "@/lib/hooks/use-metrics";

export default function DashboardPage() {
  const { latest, history, connected } = useMetricsWebSocket();
  const { data: sysInfo } = useSystemInfo();

  // CPU History aus WebSocket-Verlauf
  const cpuHistory = history.map((s) => s.cpu.usage_percent);
  const ramHistory = history.map((s) => s.ram.usage_percent);

  // Status-Farbe basierend auf Auslastung
  function cpuStatus(pct: number) {
    if (pct > 90) return "critical" as const;
    if (pct > 70) return "warning" as const;
    return "ok" as const;
  }
  function ramStatus(pct: number) {
    if (pct > 90) return "critical" as const;
    if (pct > 80) return "warning" as const;
    return "ok" as const;
  }
  function diskStatus(pct: number) {
    if (pct > 90) return "critical" as const;
    if (pct > 80) return "warning" as const;
    return "ok" as const;
  }

  const cpu = latest?.cpu;
  const ram = latest?.ram;
  const disk = latest?.disks?.[0]; // Primärer Disk
  const net = latest?.networks?.find(n => n.type === "physical" && n.link_state === "up");
  const load = latest?.load_avg;

  return (
    <div className="dashboard">
      {/* Header */}
      <div className="dashboard-header">
        <div>
          <h1 className="dashboard-title">Dashboard</h1>
          <p className="dashboard-subtitle">
            {sysInfo?.system ? (
              <>
                {sysInfo.system.cpu_model} &middot; {sysInfo.system.cpu_cores} Cores &middot; {formatBytes(sysInfo.system.ram_total_bytes)} RAM
              </>
            ) : (
              "System-Informationen werden geladen…"
            )}
          </p>
        </div>
        <div className={`ws-indicator ${connected ? "ws-indicator--on" : "ws-indicator--off"}`}>
          <span className="ws-dot" />
          {connected ? "Live" : "Verbinde…"}
        </div>
      </div>

      {/* Metriken-Grid */}
      <div className="metrics-grid">
        {/* CPU */}
        <MetricCard
          label="CPU"
          icon={<Cpu size={12} />}
          value={cpu ? `${cpu.usage_percent.toFixed(1)}%` : "—"}
          subValue={load ? `Load: ${load.load_1.toFixed(2)} / ${load.load_5.toFixed(2)} / ${load.load_15.toFixed(2)}` : undefined}
          percent={cpu?.usage_percent}
          history={cpuHistory}
          status={cpu ? cpuStatus(cpu.usage_percent) : "ok"}
        />

        {/* RAM */}
        <MetricCard
          label="RAM"
          icon={<MemoryStick size={12} />}
          value={ram ? formatBytes(ram.used_bytes) : "—"}
          subValue={ram ? `von ${formatBytes(ram.total_bytes)} · Cache: ${formatBytes(ram.cached_bytes)}` : undefined}
          percent={ram?.usage_percent}
          history={ramHistory}
          status={ram ? ramStatus(ram.usage_percent) : "ok"}
        />

        {/* Disk */}
        <MetricCard
          label="Disk"
          icon={<HardDrive size={12} />}
          value={disk ? formatBytes(disk.used_bytes) : "—"}
          subValue={disk ? `von ${formatBytes(disk.total_bytes)} · ${disk.mount_point}` : undefined}
          percent={disk?.usage_percent}
          status={disk ? diskStatus(disk.usage_percent) : "ok"}
        />

        {/* Netzwerk */}
        <MetricCard
          label="Netzwerk"
          icon={<Network size={12} />}
          value={net ? `↓ ${formatBytesPerSec(net.rx_bytes_per_sec)}` : "—"}
          subValue={net ? `↑ ${formatBytesPerSec(net.tx_bytes_per_sec)} · ${net.interface}` : undefined}
        />

        {/* Uptime */}
        <MetricCard
          label="Uptime"
          icon={<Timer size={12} />}
          value={latest ? formatUptime(latest.uptime_sec) : "—"}
          subValue={sysInfo?.system?.kernel_version ? `Kernel ${sysInfo.system.kernel_version}` : undefined}
        />

        {/* System */}
        <MetricCard
          label="System"
          icon={<Server size={12} />}
          value={sysInfo?.system?.hostname ?? "—"}
          subValue={sysInfo?.system?.os ?? undefined}
        />
      </div>

      {/* Swap (wenn vorhanden) */}
      {ram && ram.swap_total_bytes > 0 && (
        <div className="section">
          <h2 className="section-title">Swap</h2>
          <div className="metrics-grid metrics-grid--small">
            <MetricCard
              label="Swap"
              value={formatBytes(ram.swap_used_bytes)}
              subValue={`von ${formatBytes(ram.swap_total_bytes)}`}
              percent={(ram.swap_used_bytes / ram.swap_total_bytes) * 100}
              status={ram.swap_used_bytes / ram.swap_total_bytes > 0.5 ? "warning" : "ok"}
            />
          </div>
        </div>
      )}

      {/* Alle Disks */}
      {latest?.disks && latest.disks.length > 1 && (
        <div className="section">
          <h2 className="section-title">Dateisysteme</h2>
          <div className="metrics-grid">
            {latest.disks.map((d) => (
              <MetricCard
                key={d.mount_point}
                label={d.mount_point}
                icon={<HardDrive size={12} />}
                value={formatBytes(d.used_bytes)}
                subValue={`von ${formatBytes(d.total_bytes)} · ${d.fs_type} · ↓${formatBytesPerSec(d.read_bytes_per_sec)} ↑${formatBytesPerSec(d.write_bytes_per_sec)}`}
                percent={d.usage_percent}
                status={diskStatus(d.usage_percent)}
              />
            ))}
          </div>
        </div>
      )}

      {/* Netzwerk-Interfaces */}
      {latest?.networks && latest.networks.length > 0 && (
        <div className="section">
          <h2 className="section-title">Netzwerk-Interfaces</h2>
          <div className="net-table">
            <div className="net-table__header">
              <span>Interface</span>
              <span>Typ</span>
              <span>Status</span>
              <span>RX</span>
              <span>TX</span>
            </div>
            {latest.networks.map((n) => (
              <div key={n.interface} className="net-table__row">
                <span className="net-table__iface">{n.interface}</span>
                <span className="net-table__type">{n.type}</span>
                <span className={`net-table__state net-table__state--${n.link_state}`}>
                  {n.link_state}
                </span>
                <span className="net-table__bytes">{formatBytesPerSec(n.rx_bytes_per_sec)}</span>
                <span className="net-table__bytes">{formatBytesPerSec(n.tx_bytes_per_sec)}</span>
              </div>
            ))}
          </div>
        </div>
      )}

      {/* Docker */}
      {sysInfo?.system?.docker?.available && sysInfo.system.docker.containers && (
        <div className="section">
          <h2 className="section-title">
            Docker
            <span className="section-meta">
              {sysInfo.system.docker.version} · {sysInfo.system.docker.endpoint}
            </span>
          </h2>
          <div className="docker-grid">
            {sysInfo.system.docker.containers.map((c) => (
              <div key={c.id} className={`docker-card docker-card--${c.state}`}>
                <div className="docker-card__name">{c.name}</div>
                <div className="docker-card__image">{c.image}</div>
                <div className="docker-card__status">{c.status}</div>
                {c.ports && c.ports.length > 0 && (
                  <div className="docker-card__ports">
                    {c.ports.join(", ")}
                  </div>
                )}
              </div>
            ))}
          </div>
        </div>
      )}

      <style>{`
        .dashboard {
          display: flex;
          flex-direction: column;
          gap: 1.5rem;
        }

        .dashboard-header {
          display: flex;
          align-items: flex-start;
          justify-content: space-between;
          gap: 1rem;
        }

        .dashboard-title {
          font-size: 1.125rem;
          font-weight: 600;
          color: var(--text);
          margin: 0 0 0.25rem;
        }

        .dashboard-subtitle {
          font-size: 0.75rem;
          color: var(--muted);
          margin: 0;
          font-family: "JetBrains Mono", monospace;
        }

        .ws-indicator {
          display: flex;
          align-items: center;
          gap: 0.375rem;
          font-size: 0.6875rem;
          font-weight: 600;
          letter-spacing: 0.05em;
          color: var(--muted);
          padding: 0.25rem 0.625rem;
          border-radius: 999px;
          border: 0.5px solid var(--border);
        }

        .ws-indicator--on {
          color: var(--green);
          border-color: color-mix(in srgb, var(--green) 30%, transparent);
          background: color-mix(in srgb, var(--green) 8%, transparent);
        }

        .ws-dot {
          width: 6px;
          height: 6px;
          border-radius: 50%;
          background: currentColor;
        }

        .ws-indicator--on .ws-dot {
          animation: ws-blink 1.5s ease-in-out infinite;
        }

        @keyframes ws-blink {
          0%, 100% { opacity: 1; }
          50% { opacity: 0.3; }
        }

        .metrics-grid {
          display: grid;
          grid-template-columns: repeat(auto-fill, minmax(220px, 1fr));
          gap: 0.875rem;
        }

        .metrics-grid--small {
          grid-template-columns: repeat(auto-fill, minmax(180px, 1fr));
        }

        .section {
          display: flex;
          flex-direction: column;
          gap: 0.75rem;
        }

        .section-title {
          font-size: 0.75rem;
          font-weight: 600;
          color: var(--muted);
          text-transform: uppercase;
          letter-spacing: 0.07em;
          margin: 0;
          display: flex;
          align-items: center;
          gap: 0.75rem;
        }

        .section-meta {
          font-weight: 400;
          text-transform: none;
          letter-spacing: 0;
          color: var(--dim);
          font-family: "JetBrains Mono", monospace;
        }

        /* Netzwerk-Tabelle */
        .net-table {
          background: var(--surface);
          border: 0.5px solid var(--border);
          border-radius: var(--radius);
          overflow: hidden;
        }

        .net-table__header,
        .net-table__row {
          display: grid;
          grid-template-columns: 2fr 1fr 1fr 1.5fr 1.5fr;
          padding: 0.5rem 1rem;
          font-size: 0.75rem;
          gap: 0.5rem;
          align-items: center;
        }

        .net-table__header {
          background: var(--bg);
          color: var(--muted);
          font-weight: 600;
          text-transform: uppercase;
          letter-spacing: 0.05em;
          font-size: 0.6875rem;
          border-bottom: 0.5px solid var(--border-sub);
        }

        .net-table__row:not(:last-child) {
          border-bottom: 0.5px solid var(--border-sub);
        }

        .net-table__iface {
          font-family: "JetBrains Mono", monospace;
          color: var(--text);
          font-weight: 500;
        }

        .net-table__type {
          color: var(--muted);
          font-size: 0.6875rem;
        }

        .net-table__state {
          font-size: 0.6875rem;
          font-weight: 600;
          padding: 0.125rem 0.5rem;
          border-radius: 4px;
          text-align: center;
          width: fit-content;
        }

        .net-table__state--up {
          background: color-mix(in srgb, var(--green) 12%, transparent);
          color: var(--green);
        }

        .net-table__state--down {
          background: color-mix(in srgb, var(--red) 12%, transparent);
          color: var(--red);
        }

        .net-table__state--unknown {
          background: var(--border-sub);
          color: var(--muted);
        }

        .net-table__bytes {
          font-family: "JetBrains Mono", monospace;
          color: var(--text);
          font-size: 0.75rem;
        }

        /* Docker Grid */
        .docker-grid {
          display: grid;
          grid-template-columns: repeat(auto-fill, minmax(200px, 1fr));
          gap: 0.75rem;
        }

        .docker-card {
          background: var(--surface);
          border: 0.5px solid var(--border);
          border-radius: var(--radius);
          padding: 0.75rem 1rem;
          display: flex;
          flex-direction: column;
          gap: 0.25rem;
        }

        .docker-card--running {
          border-left: 2px solid var(--green);
        }

        .docker-card--exited,
        .docker-card--stopped {
          border-left: 2px solid var(--muted);
          opacity: 0.7;
        }

        .docker-card--paused {
          border-left: 2px solid var(--yellow);
        }

        .docker-card__name {
          font-size: 0.8125rem;
          font-weight: 600;
          color: var(--text);
        }

        .docker-card__image {
          font-size: 0.6875rem;
          color: var(--muted);
          font-family: "JetBrains Mono", monospace;
          white-space: nowrap;
          overflow: hidden;
          text-overflow: ellipsis;
        }

        .docker-card__status {
          font-size: 0.6875rem;
          color: var(--dim);
          font-family: "JetBrains Mono", monospace;
        }

        .docker-card__ports {
          font-size: 0.6875rem;
          color: var(--brand-t);
          font-family: "JetBrains Mono", monospace;
          margin-top: 0.25rem;
        }
      `}</style>
    </div>
  );
}
