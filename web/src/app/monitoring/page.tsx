"use client";

import type { Metadata } from "next";
import { useState } from "react";
import { Cpu, MemoryStick, HardDrive, Network, RefreshCw, X } from "lucide-react";
import { Sparkline } from "@/components/dashboard/sparkline";
import {
  useMetricsWebSocket,
  useProcesses,
  useKillProcess,
  formatBytes,
  formatBytesPerSec,
} from "@/lib/hooks/use-metrics";
import { useRole } from "@/lib/hooks/use-auth";

export default function MonitoringPage() {
  const { latest, history, connected } = useMetricsWebSocket();
  const role = useRole();
  const [processSort, setProcessSort] = useState("mem");
  const { data: procData, isLoading: procsLoading } = useProcesses(processSort);
  const killProcess = useKillProcess();
  const [killConfirm, setKillConfirm] = useState<number | null>(null);

  const cpuHistory = history.map((s) => s.cpu.usage_percent);
  const ramHistory = history.map((s) => s.ram.usage_percent);

  function handleKill(pid: number) {
    if (killConfirm === pid) {
      killProcess.mutate(pid);
      setKillConfirm(null);
    } else {
      setKillConfirm(pid);
      setTimeout(() => setKillConfirm(null), 3000);
    }
  }

  return (
    <div className="monitoring">
      <div className="monitoring-header">
        <h1 className="page-title">Monitoring</h1>
        <div className={`ws-pill ${connected ? "ws-pill--on" : ""}`}>
          <span className="ws-dot" />
          {connected ? "Live" : "Offline"}
        </div>
      </div>

      {/* CPU Detail */}
      <section className="mon-section">
        <h2 className="mon-section__title">
          <Cpu size={14} /> CPU
          {latest?.cpu && (
            <span className="mon-section__value">
              {latest.cpu.usage_percent.toFixed(1)}%
            </span>
          )}
        </h2>

        {/* Gesamt-Sparkline */}
        <div className="chart-card">
          <div className="chart-card__label">Gesamt (60s)</div>
          <Sparkline
            data={cpuHistory}
            width={600}
            height={60}
            color="var(--brand-l)"
            max={100}
            className="chart-full"
          />
        </div>

        {/* Pro-Core */}
        {latest?.cpu.core_usage_percent && (
          <div className="core-grid">
            {latest.cpu.core_usage_percent.map((pct, i) => (
              <div key={i} className="core-card">
                <div className="core-card__label">Core {i}</div>
                <div className="core-card__value">{pct.toFixed(1)}%</div>
                <div className="core-bar-track">
                  <div
                    className="core-bar-fill"
                    style={{
                      width: `${pct}%`,
                      background: pct > 90 ? "var(--red)" : pct > 70 ? "var(--yellow)" : "var(--brand-l)",
                    }}
                  />
                </div>
              </div>
            ))}
          </div>
        )}
      </section>

      {/* RAM Detail */}
      <section className="mon-section">
        <h2 className="mon-section__title">
          <MemoryStick size={14} /> RAM
          {latest?.ram && (
            <span className="mon-section__value">
              {formatBytes(latest.ram.used_bytes)} / {formatBytes(latest.ram.total_bytes)}
            </span>
          )}
        </h2>

        <div className="chart-card">
          <div className="chart-card__label">Auslastung (60s)</div>
          <Sparkline
            data={ramHistory}
            width={600}
            height={60}
            color="var(--green)"
            max={100}
            className="chart-full"
          />
        </div>

        {latest?.ram && (
          <div className="ram-breakdown">
            {[
              { label: "Used",    value: latest.ram.used_bytes,    color: "var(--brand-l)" },
              { label: "Cache",   value: latest.ram.cached_bytes,  color: "var(--green)" },
              { label: "Free",    value: latest.ram.free_bytes,    color: "var(--muted)" },
              { label: "Swap",    value: latest.ram.swap_used_bytes, color: "var(--yellow)" },
            ].map(({ label, value, color }) => (
              <div key={label} className="ram-item">
                <div className="ram-item__dot" style={{ background: color }} />
                <span className="ram-item__label">{label}</span>
                <span className="ram-item__value">{formatBytes(value)}</span>
              </div>
            ))}
          </div>
        )}
      </section>

      {/* Disk I/O */}
      {latest?.disks && latest.disks.length > 0 && (
        <section className="mon-section">
          <h2 className="mon-section__title">
            <HardDrive size={14} /> Disk I/O
          </h2>
          <div className="io-grid">
            {latest.disks.map((d) => (
              <div key={d.mount_point} className="io-card">
                <div className="io-card__device">{d.mount_point}</div>
                <div className="io-card__stats">
                  <span className="io-read">↓ {formatBytesPerSec(d.read_bytes_per_sec)}</span>
                  <span className="io-write">↑ {formatBytesPerSec(d.write_bytes_per_sec)}</span>
                </div>
                <div className="io-bar-track">
                  <div className="io-bar-fill" style={{ width: `${d.usage_percent}%` }} />
                </div>
                <div className="io-usage">
                  {formatBytes(d.used_bytes)} / {formatBytes(d.total_bytes)} ({d.usage_percent.toFixed(1)}%)
                </div>
              </div>
            ))}
          </div>
        </section>
      )}

      {/* Prozesse */}
      <section className="mon-section">
        <h2 className="mon-section__title">
          <RefreshCw size={14} /> Prozesse
          {procData && <span className="mon-section__meta">{procData.total} gesamt</span>}
        </h2>

        <div className="proc-controls">
          {["mem", "cpu", "pid"].map((s) => (
            <button
              key={s}
              className={`proc-sort-btn ${processSort === s ? "proc-sort-btn--active" : ""}`}
              onClick={() => setProcessSort(s)}
            >
              {s === "mem" ? "RAM" : s === "cpu" ? "CPU" : "PID"}
            </button>
          ))}
        </div>

        <div className="proc-table">
          <div className="proc-table__header">
            <span>PID</span>
            <span>Name</span>
            <span>Status</span>
            <span>RAM</span>
            <span>RAM %</span>
            {role === "admin" && <span></span>}
          </div>

          {procsLoading ? (
            <div className="proc-loading">Lade Prozesse…</div>
          ) : (
            procData?.processes.map((p) => (
              <div key={p.pid} className="proc-table__row">
                <span className="proc-pid">{p.pid}</span>
                <span className="proc-name">{p.name}</span>
                <span className={`proc-status proc-status--${p.status}`}>{p.status}</span>
                <span className="proc-mem">{formatBytes(p.mem_bytes)}</span>
                <span className="proc-pct">{p.mem_percent.toFixed(1)}%</span>
                {role === "admin" && (
                  <button
                    className={`proc-kill ${killConfirm === p.pid ? "proc-kill--confirm" : ""}`}
                    onClick={() => handleKill(p.pid)}
                    title={killConfirm === p.pid ? "Nochmals klicken zum Bestätigen" : "Prozess beenden (SIGTERM)"}
                  >
                    <X size={12} />
                  </button>
                )}
              </div>
            ))
          )}
        </div>
      </section>

      <style>{`
        .monitoring { display: flex; flex-direction: column; gap: 1.5rem; }

        .monitoring-header {
          display: flex;
          align-items: center;
          justify-content: space-between;
        }

        .page-title {
          font-size: 1.125rem;
          font-weight: 600;
          color: var(--text);
          margin: 0;
        }

        .ws-pill {
          display: flex;
          align-items: center;
          gap: 0.375rem;
          font-size: 0.6875rem;
          font-weight: 600;
          color: var(--muted);
          padding: 0.2rem 0.5rem;
          border-radius: 999px;
          border: 0.5px solid var(--border);
        }
        .ws-pill--on { color: var(--green); border-color: color-mix(in srgb, var(--green) 30%, transparent); }
        .ws-dot { width: 5px; height: 5px; border-radius: 50%; background: currentColor; }
        .ws-pill--on .ws-dot { animation: blink 1.5s ease-in-out infinite; }
        @keyframes blink { 0%,100%{opacity:1} 50%{opacity:0.3} }

        .mon-section {
          background: var(--surface);
          border: 0.5px solid var(--border);
          border-radius: var(--radius);
          padding: 1rem 1.25rem;
          display: flex;
          flex-direction: column;
          gap: 0.875rem;
        }

        .mon-section__title {
          font-size: 0.75rem;
          font-weight: 600;
          color: var(--muted);
          text-transform: uppercase;
          letter-spacing: 0.07em;
          margin: 0;
          display: flex;
          align-items: center;
          gap: 0.5rem;
        }

        .mon-section__value {
          font-family: "JetBrains Mono", monospace;
          color: var(--text);
          font-weight: 700;
          font-size: 0.875rem;
          letter-spacing: -0.02em;
          text-transform: none;
          letter-spacing: 0;
        }

        .mon-section__meta { color: var(--dim); font-weight: 400; text-transform: none; letter-spacing: 0; }

        .chart-card {
          display: flex;
          flex-direction: column;
          gap: 0.375rem;
        }

        .chart-card__label { font-size: 0.6875rem; color: var(--dim); }
        .chart-full { width: 100% !important; }

        /* CPU Cores */
        .core-grid {
          display: grid;
          grid-template-columns: repeat(auto-fill, minmax(100px, 1fr));
          gap: 0.5rem;
        }

        .core-card {
          background: var(--bg);
          border: 0.5px solid var(--border-sub);
          border-radius: 6px;
          padding: 0.5rem 0.625rem;
        }

        .core-card__label { font-size: 0.625rem; color: var(--dim); margin-bottom: 0.2rem; }
        .core-card__value {
          font-family: "JetBrains Mono", monospace;
          font-size: 0.875rem;
          font-weight: 700;
          color: var(--text);
          margin-bottom: 0.375rem;
        }

        .core-bar-track { height: 3px; background: var(--border-sub); border-radius: 2px; overflow: hidden; }
        .core-bar-fill { height: 100%; border-radius: 2px; transition: width 0.5s ease; }

        /* RAM Breakdown */
        .ram-breakdown {
          display: flex;
          gap: 1.25rem;
          flex-wrap: wrap;
        }

        .ram-item { display: flex; align-items: center; gap: 0.375rem; }
        .ram-item__dot { width: 8px; height: 8px; border-radius: 2px; flex-shrink: 0; }
        .ram-item__label { font-size: 0.6875rem; color: var(--muted); }
        .ram-item__value { font-family: "JetBrains Mono", monospace; font-size: 0.75rem; color: var(--text); font-weight: 600; }

        /* Disk I/O */
        .io-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(220px, 1fr)); gap: 0.75rem; }

        .io-card {
          background: var(--bg);
          border: 0.5px solid var(--border-sub);
          border-radius: 6px;
          padding: 0.75rem;
          display: flex;
          flex-direction: column;
          gap: 0.375rem;
        }

        .io-card__device { font-family: "JetBrains Mono", monospace; font-size: 0.8125rem; font-weight: 600; color: var(--text); }
        .io-card__stats { display: flex; gap: 0.75rem; font-family: "JetBrains Mono", monospace; font-size: 0.75rem; }
        .io-read { color: var(--green); }
        .io-write { color: var(--brand-l); }
        .io-bar-track { height: 3px; background: var(--border-sub); border-radius: 2px; overflow: hidden; }
        .io-bar-fill { height: 100%; background: var(--brand); border-radius: 2px; transition: width 0.5s; }
        .io-usage { font-size: 0.6875rem; color: var(--dim); font-family: "JetBrains Mono", monospace; }

        /* Prozesse */
        .proc-controls { display: flex; gap: 0.375rem; }

        .proc-sort-btn {
          background: var(--bg);
          border: 0.5px solid var(--border);
          color: var(--muted);
          font-size: 0.6875rem;
          font-weight: 600;
          padding: 0.25rem 0.625rem;
          border-radius: 4px;
          cursor: pointer;
          transition: all 0.15s;
        }

        .proc-sort-btn--active {
          background: var(--brand-bg);
          color: var(--brand-t);
          border-color: var(--brand);
        }

        .proc-table {
          border: 0.5px solid var(--border-sub);
          border-radius: 6px;
          overflow: hidden;
        }

        .proc-table__header,
        .proc-table__row {
          display: grid;
          grid-template-columns: 60px 1fr 70px 80px 60px 32px;
          padding: 0.4rem 0.75rem;
          font-size: 0.75rem;
          gap: 0.5rem;
          align-items: center;
        }

        .proc-table__header {
          background: var(--bg);
          color: var(--muted);
          font-size: 0.6875rem;
          font-weight: 600;
          text-transform: uppercase;
          letter-spacing: 0.04em;
          border-bottom: 0.5px solid var(--border-sub);
        }

        .proc-table__row:not(:last-child) { border-bottom: 0.5px solid var(--border-sub); }
        .proc-table__row:hover { background: var(--border-sub); }

        .proc-pid { font-family: "JetBrains Mono", monospace; color: var(--dim); font-size: 0.6875rem; }
        .proc-name { color: var(--text); font-weight: 500; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
        .proc-mem, .proc-pct { font-family: "JetBrains Mono", monospace; color: var(--muted); font-size: 0.6875rem; }

        .proc-status {
          font-size: 0.6875rem;
          padding: 0.125rem 0.375rem;
          border-radius: 3px;
          font-weight: 600;
          text-align: center;
        }

        .proc-status--S { background: color-mix(in srgb, var(--green) 12%, transparent); color: var(--green); }
        .proc-status--R { background: color-mix(in srgb, var(--brand-l) 12%, transparent); color: var(--brand-l); }
        .proc-status--Z { background: color-mix(in srgb, var(--red) 12%, transparent); color: var(--red); }
        .proc-status--D { background: color-mix(in srgb, var(--yellow) 12%, transparent); color: var(--yellow); }

        .proc-kill {
          background: none;
          border: 0.5px solid transparent;
          color: var(--dim);
          cursor: pointer;
          display: flex;
          align-items: center;
          justify-content: center;
          width: 24px;
          height: 24px;
          border-radius: 4px;
          transition: all 0.15s;
        }

        .proc-kill:hover { color: var(--red); border-color: color-mix(in srgb, var(--red) 40%, transparent); }
        .proc-kill--confirm { color: var(--red); border-color: var(--red); background: color-mix(in srgb, var(--red) 10%, transparent); animation: confirm-pulse 0.5s ease; }

        @keyframes confirm-pulse { 0%,100%{transform:scale(1)} 50%{transform:scale(1.15)} }

        .proc-loading { padding: 1rem; text-align: center; color: var(--muted); font-size: 0.8125rem; }
      `}</style>
    </div>
  );
}
