"use client";

import { useState, useEffect, useRef, useCallback } from "react";
import { useQuery } from "@tanstack/react-query";
import {
  Play, Pause, Download, Search, X, Filter,
  AlertOctagon, AlertTriangle, Info, Bug
} from "lucide-react";
import { apiClient } from "@/lib/api/client";
import { tokenStore } from "@/lib/api/auth";
import { cn } from "@/lib/utils/cn";

interface LogEntry {
  timestamp: string;
  severity: string;
  unit: string;
  message: string;
  source: string;
  pid?: string;
  is_new?: boolean;
}

interface LogSource {
  id: string;
  label: string;
  path?: string;
}

const SEVERITY_COLORS: Record<string, string> = {
  error:   "var(--red)",
  warning: "var(--yellow)",
  info:    "var(--text)",
  debug:   "var(--dim)",
};

const SEVERITY_ICONS: Record<string, React.ComponentType<{ size?: number }>> = {
  error:   AlertOctagon,
  warning: AlertTriangle,
  info:    Info,
  debug:   Bug,
};

export default function LogsPage() {
  // Filter-State
  const [source, setSource]     = useState("journald");
  const [unit, setUnit]         = useState("");
  const [severity, setSeverity] = useState("");
  const [search, setSearch]     = useState("");
  const [lines, setLines]       = useState(100);

  // Live-Tail-State
  const [live, setLive]         = useState(false);
  const [liveEntries, setLiveEntries] = useState<LogEntry[]>([]);
  const wsRef                   = useRef<WebSocket | null>(null);
  const bottomRef               = useRef<HTMLDivElement>(null);
  const [autoScroll, setAutoScroll] = useState(true);

  // Quellen + Units laden
  const { data: sources } = useQuery<LogSource[]>({
    queryKey: ["logs", "sources"],
    queryFn: () => apiClient.get<LogSource[]>("/logs/sources"),
    staleTime: 60_000,
  });

  const { data: units } = useQuery<string[]>({
    queryKey: ["logs", "units"],
    queryFn: () => apiClient.get<string[]>("/logs/units"),
    staleTime: 30_000,
  });

  // Log-Einträge laden
  const { data, isLoading, refetch } = useQuery({
    queryKey: ["logs", source, unit, severity, search, lines],
    queryFn: () => apiClient.get<{ entries: LogEntry[]; count: number }>(
      `/logs?source=${source}&unit=${encodeURIComponent(unit)}&severity=${severity}&search=${encodeURIComponent(search)}&lines=${lines}`
    ),
    enabled: !live,
    staleTime: 10_000,
  });

  // Live-Tail WebSocket
  const startLive = useCallback(() => {
    const token = tokenStore.getAccess();
    if (!token) return;

    const protocol = window.location.protocol === "https:" ? "wss:" : "ws:";
    const host = process.env.NEXT_PUBLIC_WS_HOST ?? window.location.host;
    const unitParam = unit ? `&unit=${encodeURIComponent(unit)}` : "";
    const url = `${protocol}//${host}/ws/logs?token=${token}${unitParam}`;

    const ws = new WebSocket(url);
    wsRef.current = ws;

    ws.onmessage = (event) => {
      try {
        const msg = JSON.parse(event.data);
        if (msg.type === "log") {
          setLiveEntries((prev) => {
            const next = [...prev, msg.data as LogEntry];
            return next.slice(-500); // Max 500 Einträge
          });
        }
      } catch { /* ignore */ }
    };

    ws.onclose = () => setLive(false);
    setLive(true);
    setLiveEntries([]);
  }, [unit]);

  const stopLive = useCallback(() => {
    wsRef.current?.close();
    wsRef.current = null;
    setLive(false);
  }, []);

  // Auto-Scroll
  useEffect(() => {
    if (autoScroll && live) {
      bottomRef.current?.scrollIntoView({ behavior: "smooth" });
    }
  }, [liveEntries, autoScroll, live]);

  // Export
  function handleExport(format: "txt" | "json") {
    const params = new URLSearchParams({ source, unit, severity, search, format });
    window.location.href = `/api/v1/logs/export?${params}`;
  }

  const displayEntries = live ? liveEntries : (data?.entries ?? []);

  return (
    <div className="logs-page">
      {/* Header */}
      <div className="logs-header">
        <div>
          <h1 className="page-title">Logs</h1>
          <p className="page-subtitle">
            {live ? (
              <span className="live-indicator">● Live-Tail aktiv</span>
            ) : (
              `${data?.count ?? 0} Einträge`
            )}
          </p>
        </div>

        <div className="logs-actions">
          <button
            className={cn("action-btn", live ? "action-btn--danger" : "action-btn--primary")}
            onClick={live ? stopLive : startLive}
          >
            {live ? <><Pause size={13} /> Stop</> : <><Play size={13} /> Live-Tail</>}
          </button>
          <button className="action-btn" onClick={() => handleExport("txt")}>
            <Download size={13} /> TXT
          </button>
          <button className="action-btn" onClick={() => handleExport("json")}>
            <Download size={13} /> JSON
          </button>
        </div>
      </div>

      {/* Filter-Leiste */}
      <div className="logs-filters">
        {/* Quelle */}
        <select className="filter-select" value={source} onChange={(e) => setSource(e.target.value)}>
          {sources?.map((s) => (
            <option key={s.id} value={s.id}>{s.label}</option>
          ))}
        </select>

        {/* Unit (nur journald) */}
        {source === "journald" && (
          <select className="filter-select" value={unit} onChange={(e) => setUnit(e.target.value)}>
            <option value="">Alle Units</option>
            {units?.map((u) => (
              <option key={u} value={u}>{u}</option>
            ))}
          </select>
        )}

        {/* Severity */}
        <select className="filter-select" value={severity} onChange={(e) => setSeverity(e.target.value)}>
          <option value="">Alle Level</option>
          <option value="error">Error</option>
          <option value="warning">Warning</option>
          <option value="info">Info</option>
          <option value="debug">Debug</option>
        </select>

        {/* Suche */}
        <div className="filter-search">
          <Search size={13} className="filter-search__icon" />
          <input
            type="text"
            placeholder="Suchen…"
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            className="filter-search__input"
          />
          {search && (
            <button className="filter-search__clear" onClick={() => setSearch("")}>
              <X size={12} />
            </button>
          )}
        </div>

        {/* Lines */}
        {!live && (
          <select
            className="filter-select filter-select--narrow"
            value={lines}
            onChange={(e) => setLines(Number(e.target.value))}
          >
            <option value={50}>50</option>
            <option value={100}>100</option>
            <option value={200}>200</option>
            <option value={500}>500</option>
          </select>
        )}

        {/* Active filter tags */}
        {(unit || severity || search) && (
          <div className="filter-tags">
            {unit && <Tag label={unit} onRemove={() => setUnit("")} />}
            {severity && <Tag label={severity} onRemove={() => setSeverity("")} />}
            {search && <Tag label={`"${search}"`} onRemove={() => setSearch("")} />}
            <button className="filter-clear-all" onClick={() => { setUnit(""); setSeverity(""); setSearch(""); }}>
              <Filter size={11} /> Alle Filter
            </button>
          </div>
        )}
      </div>

      {/* Live-Scroll-Kontrolle */}
      {live && (
        <div className="live-controls">
          <label className="live-autoscroll">
            <input type="checkbox" checked={autoScroll} onChange={(e) => setAutoScroll(e.target.checked)} />
            Auto-Scroll
          </label>
          <span className="live-count">{liveEntries.length} / 500 Einträge im Buffer</span>
        </div>
      )}

      {/* Log-Tabelle */}
      <div className="log-table">
        {isLoading && !live && (
          <div className="log-loading">Logs werden geladen…</div>
        )}

        {displayEntries.length === 0 && !isLoading && (
          <div className="log-empty">
            {live ? "Warte auf neue Log-Einträge…" : "Keine Einträge gefunden."}
          </div>
        )}

        {displayEntries.map((entry, i) => (
          <LogRow key={`${entry.timestamp}-${i}`} entry={entry} />
        ))}

        <div ref={bottomRef} />
      </div>

      <style>{`
        .logs-page { display: flex; flex-direction: column; gap: 1rem; height: calc(100dvh - 44px - 2.5rem); }

        .logs-header {
          display: flex; align-items: flex-start; justify-content: space-between; flex-shrink: 0;
        }

        .page-title { font-size: 1.125rem; font-weight: 600; color: var(--text); margin: 0 0 0.25rem; }
        .page-subtitle { font-size: 0.75rem; color: var(--muted); margin: 0; }
        .live-indicator { color: var(--red); font-weight: 600; animation: pulse 1s ease-in-out infinite; }
        @keyframes pulse { 0%,100%{opacity:1} 50%{opacity:0.5} }

        .logs-actions { display: flex; gap: 0.5rem; }

        .action-btn {
          display: flex; align-items: center; gap: 0.375rem;
          background: var(--surface); border: 0.5px solid var(--border);
          color: var(--muted); font-size: 0.75rem; font-weight: 500;
          padding: 0.375rem 0.75rem; border-radius: 6px; cursor: pointer;
          transition: all 0.15s;
        }
        .action-btn:hover { color: var(--text); border-color: var(--brand); }
        .action-btn--primary { background: var(--brand); color: white; border-color: var(--brand); }
        .action-btn--primary:hover { background: var(--brand-l); border-color: var(--brand-l); }
        .action-btn--danger { background: color-mix(in srgb, var(--red) 15%, transparent); color: var(--red); border-color: color-mix(in srgb, var(--red) 40%, transparent); }

        .logs-filters {
          display: flex; align-items: center; gap: 0.5rem; flex-wrap: wrap; flex-shrink: 0;
        }

        .filter-select {
          background: var(--surface); border: 0.5px solid var(--border); color: var(--text);
          font-size: 0.75rem; padding: 0.375rem 0.625rem; border-radius: 6px; outline: none;
          cursor: pointer; transition: border-color 0.15s;
        }
        .filter-select:focus { border-color: var(--brand-l); }
        .filter-select--narrow { width: 72px; }

        .filter-search {
          display: flex; align-items: center; gap: 0.375rem;
          background: var(--surface); border: 0.5px solid var(--border);
          border-radius: 6px; padding: 0 0.625rem; transition: border-color 0.15s;
          flex: 1; min-width: 160px; max-width: 280px;
        }
        .filter-search:focus-within { border-color: var(--brand-l); }
        .filter-search__icon { color: var(--muted); flex-shrink: 0; }
        .filter-search__input {
          background: none; border: none; color: var(--text); font-size: 0.75rem;
          padding: 0.375rem 0; outline: none; width: 100%;
        }
        .filter-search__input::placeholder { color: var(--dim); }
        .filter-search__clear { background: none; border: none; color: var(--muted); cursor: pointer; display:flex; }

        .filter-tags { display: flex; align-items: center; gap: 0.375rem; }
        .filter-clear-all {
          display: flex; align-items: center; gap: 0.25rem;
          background: none; border: none; color: var(--dim); font-size: 0.6875rem;
          cursor: pointer; transition: color 0.15s;
        }
        .filter-clear-all:hover { color: var(--muted); }

        .live-controls {
          display: flex; align-items: center; justify-content: space-between;
          flex-shrink: 0; padding: 0.375rem 0.75rem;
          background: color-mix(in srgb, var(--red) 6%, transparent);
          border: 0.5px solid color-mix(in srgb, var(--red) 20%, transparent);
          border-radius: 6px;
        }
        .live-autoscroll { display: flex; align-items: center; gap: 0.5rem; font-size: 0.75rem; color: var(--muted); cursor: pointer; }
        .live-count { font-size: 0.6875rem; color: var(--dim); font-family: "JetBrains Mono", monospace; }

        .log-table {
          flex: 1; overflow-y: auto; background: var(--surface);
          border: 0.5px solid var(--border); border-radius: var(--radius);
          font-family: "JetBrains Mono", monospace; font-size: 0.75rem;
        }

        .log-loading, .log-empty {
          padding: 2rem; text-align: center; color: var(--muted); font-size: 0.8125rem;
          font-family: inherit;
        }
      `}</style>
    </div>
  );
}

function LogRow({ entry }: { entry: LogEntry }) {
  const color = SEVERITY_COLORS[entry.severity] ?? "var(--text)";
  const Icon = SEVERITY_ICONS[entry.severity] ?? Info;
  const ts = new Date(entry.timestamp).toLocaleTimeString("de-DE", {
    hour: "2-digit", minute: "2-digit", second: "2-digit"
  });

  return (
    <div className={cn("log-row", entry.is_new && "log-row--new")}>
      <span className="log-row__ts">{ts}</span>
      <span className="log-row__icon" style={{ color }}>
        <Icon size={11} />
      </span>
      <span className="log-row__unit">{entry.unit || "—"}</span>
      <span className="log-row__msg" style={{ color: entry.severity === "error" ? color : undefined }}>
        {entry.message}
      </span>

      <style>{`
        .log-row {
          display: grid;
          grid-template-columns: 72px 18px 160px 1fr;
          gap: 0.5rem;
          padding: 0.25rem 0.75rem;
          align-items: baseline;
          border-bottom: 0.5px solid var(--border-sub);
          transition: background 0.1s;
        }
        .log-row:hover { background: var(--border-sub); }
        .log-row--new { animation: log-fade-in 0.3s ease; }
        @keyframes log-fade-in { from { background: color-mix(in srgb, var(--brand) 8%, transparent); } to { background: transparent; } }

        .log-row__ts { color: var(--dim); font-size: 0.6875rem; }
        .log-row__icon { display: flex; align-items: center; }
        .log-row__unit { color: var(--muted); font-size: 0.6875rem; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
        .log-row__msg { color: var(--text); overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
      `}</style>
    </div>
  );
}

function Tag({ label, onRemove }: { label: string; onRemove: () => void }) {
  return (
    <span className="filter-tag">
      {label}
      <button className="filter-tag__remove" onClick={onRemove}><X size={10} /></button>
      <style>{`
        .filter-tag {
          display: inline-flex; align-items: center; gap: 0.25rem;
          background: var(--brand-bg); color: var(--brand-t); border: 0.5px solid var(--brand);
          border-radius: 4px; padding: 0.125rem 0.375rem; font-size: 0.6875rem; font-weight: 500;
        }
        .filter-tag__remove { background: none; border: none; color: inherit; cursor: pointer; display:flex; padding:0; }
      `}</style>
    </span>
  );
}
