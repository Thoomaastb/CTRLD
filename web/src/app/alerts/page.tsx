"use client";

import { useState } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { AlertTriangle, AlertOctagon, CheckCircle, Settings } from "lucide-react";
import { apiClient } from "@/lib/api/client";
import { useRole } from "@/lib/hooks/use-auth";

interface Alert {
  id: string;
  type: string;
  severity: string;
  threshold: number;
  current_value: number;
  message?: string;
  triggered_at: string;
  resolved_at?: string;
}

interface Threshold {
  type: string;
  warning: number;
  critical: number;
  enabled: boolean;
}

export default function AlertsPage() {
  const role = useRole();
  const qc = useQueryClient();
  const [showThresholds, setShowThresholds] = useState(false);

  const { data: activeData } = useQuery({
    queryKey: ["alerts", "active"],
    queryFn: () => apiClient.get<{ alerts: Alert[]; count: number }>("/alerts"),
    refetchInterval: 15_000,
  });

  const { data: historyData } = useQuery({
    queryKey: ["alerts", "history"],
    queryFn: () => apiClient.get<{ alerts: Alert[]; count: number }>("/alerts/history"),
    refetchInterval: 30_000,
  });

  const { data: thresholds } = useQuery({
    queryKey: ["alerts", "thresholds"],
    queryFn: () => apiClient.get<Threshold[]>("/alerts/thresholds"),
  });

  const updateThresholds = useMutation({
    mutationFn: (t: Threshold[]) => apiClient.put("/alerts/thresholds", t),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["alerts", "thresholds"] }),
  });

  const [editThresholds, setEditThresholds] = useState<Threshold[] | null>(null);

  function startEdit() {
    setEditThresholds(thresholds ? JSON.parse(JSON.stringify(thresholds)) : []);
    setShowThresholds(true);
  }

  function saveThresholds() {
    if (editThresholds) {
      updateThresholds.mutate(editThresholds);
      setShowThresholds(false);
    }
  }

  const active = activeData?.alerts ?? [];
  const history = historyData?.alerts ?? [];

  return (
    <div className="alerts-page">
      <div className="alerts-header">
        <div>
          <h1 className="page-title">Alerts</h1>
          <p className="page-subtitle">
            {active.length > 0 ? (
              <span className="active-count">{active.length} aktiv</span>
            ) : (
              <span className="all-clear">Alle Systeme normal</span>
            )}
          </p>
        </div>
        {role === "admin" && (
          <button className="threshold-btn" onClick={startEdit}>
            <Settings size={14} /> Schwellwerte
          </button>
        )}
      </div>

      {/* Aktive Alerts */}
      {active.length > 0 && (
        <section className="alerts-section">
          <h2 className="section-title">Aktiv</h2>
          <div className="alert-list">
            {active.map((a) => (
              <AlertRow key={a.id} alert={a} />
            ))}
          </div>
        </section>
      )}

      {/* History */}
      <section className="alerts-section">
        <h2 className="section-title">Verlauf</h2>
        {history.length === 0 ? (
          <div className="empty-state">Keine Alerts in der History.</div>
        ) : (
          <div className="alert-list">
            {history.map((a) => (
              <AlertRow key={a.id} alert={a} />
            ))}
          </div>
        )}
      </section>

      {/* Threshold-Editor */}
      {showThresholds && editThresholds && (
        <div className="threshold-overlay" onClick={() => setShowThresholds(false)}>
          <div className="threshold-modal" onClick={(e) => e.stopPropagation()}>
            <h3 className="modal-title">Alert-Schwellwerte</h3>
            <p className="modal-desc">Warning- und Critical-Schwellwerte in Prozent.</p>

            {editThresholds.map((t, i) => (
              <div key={t.type} className="threshold-row">
                <div className="threshold-row__type">{t.type.toUpperCase()}</div>
                <label className="threshold-field">
                  <span>Warning</span>
                  <input
                    type="number"
                    min={0} max={99}
                    value={t.warning}
                    onChange={(e) => {
                      const next = [...editThresholds];
                      next[i] = { ...t, warning: Number(e.target.value) };
                      setEditThresholds(next);
                    }}
                    className="threshold-input"
                  />
                  <span>%</span>
                </label>
                <label className="threshold-field">
                  <span>Critical</span>
                  <input
                    type="number"
                    min={1} max={100}
                    value={t.critical}
                    onChange={(e) => {
                      const next = [...editThresholds];
                      next[i] = { ...t, critical: Number(e.target.value) };
                      setEditThresholds(next);
                    }}
                    className="threshold-input"
                  />
                  <span>%</span>
                </label>
                <label className="threshold-enabled">
                  <input
                    type="checkbox"
                    checked={t.enabled}
                    onChange={(e) => {
                      const next = [...editThresholds];
                      next[i] = { ...t, enabled: e.target.checked };
                      setEditThresholds(next);
                    }}
                  />
                  Aktiv
                </label>
              </div>
            ))}

            <div className="modal-actions">
              <button className="modal-cancel" onClick={() => setShowThresholds(false)}>Abbrechen</button>
              <button className="modal-save" onClick={saveThresholds}>Speichern</button>
            </div>
          </div>
        </div>
      )}

      <style>{`
        .alerts-page { display: flex; flex-direction: column; gap: 1.5rem; }

        .alerts-header {
          display: flex;
          align-items: flex-start;
          justify-content: space-between;
        }

        .page-title { font-size: 1.125rem; font-weight: 600; color: var(--text); margin: 0 0 0.25rem; }
        .page-subtitle { font-size: 0.75rem; margin: 0; }
        .active-count { color: var(--red); font-weight: 600; }
        .all-clear { color: var(--green); font-weight: 600; }

        .threshold-btn {
          display: flex; align-items: center; gap: 0.375rem;
          background: var(--surface); border: 0.5px solid var(--border);
          color: var(--muted); font-size: 0.75rem; font-weight: 500;
          padding: 0.375rem 0.75rem; border-radius: 6px; cursor: pointer;
          transition: all 0.15s;
        }
        .threshold-btn:hover { color: var(--text); border-color: var(--brand); }

        .alerts-section { display: flex; flex-direction: column; gap: 0.625rem; }
        .section-title {
          font-size: 0.6875rem; font-weight: 600; color: var(--muted);
          text-transform: uppercase; letter-spacing: 0.07em; margin: 0;
        }

        .alert-list { display: flex; flex-direction: column; gap: 0.5rem; }

        .empty-state { color: var(--dim); font-size: 0.8125rem; padding: 1rem 0; }

        /* Threshold Modal */
        .threshold-overlay {
          position: fixed; inset: 0;
          background: rgba(0,0,0,0.6); backdrop-filter: blur(4px);
          display: flex; align-items: center; justify-content: center; z-index: 1000;
        }

        .threshold-modal {
          background: var(--surface); border: 0.5px solid var(--border);
          border-radius: var(--radius); padding: 1.5rem; width: 480px; max-width: 90vw;
        }

        .modal-title { font-size: 1rem; font-weight: 600; color: var(--text); margin: 0 0 0.25rem; }
        .modal-desc { font-size: 0.75rem; color: var(--muted); margin: 0 0 1.25rem; }

        .threshold-row {
          display: flex; align-items: center; gap: 1rem;
          padding: 0.75rem 0; border-bottom: 0.5px solid var(--border-sub);
        }
        .threshold-row:last-of-type { border-bottom: none; }
        .threshold-row__type {
          font-family: "JetBrains Mono", monospace; font-size: 0.75rem;
          font-weight: 700; color: var(--text); width: 48px;
        }

        .threshold-field {
          display: flex; align-items: center; gap: 0.375rem;
          font-size: 0.75rem; color: var(--muted);
        }

        .threshold-input {
          background: var(--bg); border: 0.5px solid var(--border);
          border-radius: 4px; color: var(--text); font-size: 0.8125rem;
          font-family: "JetBrains Mono", monospace; padding: 0.25rem 0.5rem;
          width: 56px; text-align: right; outline: none;
        }
        .threshold-input:focus { border-color: var(--brand-l); }

        .threshold-enabled {
          display: flex; align-items: center; gap: 0.375rem;
          font-size: 0.75rem; color: var(--muted); cursor: pointer; margin-left: auto;
        }

        .modal-actions { display: flex; justify-content: flex-end; gap: 0.625rem; margin-top: 1.25rem; }

        .modal-cancel {
          background: var(--bg); border: 0.5px solid var(--border);
          color: var(--muted); font-size: 0.8125rem; padding: 0.5rem 1rem;
          border-radius: 6px; cursor: pointer; transition: all 0.15s;
        }
        .modal-cancel:hover { color: var(--text); }

        .modal-save {
          background: var(--brand); border: none; color: white;
          font-size: 0.8125rem; font-weight: 600; padding: 0.5rem 1rem;
          border-radius: 6px; cursor: pointer; transition: background 0.15s;
        }
        .modal-save:hover { background: var(--brand-l); }
      `}</style>
    </div>
  );
}

function AlertRow({ alert }: { alert: Alert }) {
  const isResolved = !!alert.resolved_at;
  const isCritical = alert.severity === "critical";

  const Icon = isResolved ? CheckCircle : isCritical ? AlertOctagon : AlertTriangle;
  const color = isResolved ? "var(--green)" : isCritical ? "var(--red)" : "var(--yellow)";

  const time = new Date(alert.triggered_at).toLocaleString("de-DE", {
    day: "2-digit", month: "2-digit", year: "numeric",
    hour: "2-digit", minute: "2-digit",
  });

  return (
    <div className={`alert-row ${isResolved ? "alert-row--resolved" : ""}`}>
      <div className="alert-row__icon" style={{ color }}>
        <Icon size={14} />
      </div>
      <div className="alert-row__content">
        <div className="alert-row__title">
          <span className="alert-row__type" style={{ color }}>
            {alert.type.toUpperCase()}
          </span>
          <span className="alert-row__severity">{alert.severity}</span>
          {isResolved && <span className="alert-row__resolved-tag">aufgelöst</span>}
        </div>
        <div className="alert-row__meta">
          Wert: {alert.current_value.toFixed(1)}% · Schwellwert: {alert.threshold}% · {time}
          {alert.resolved_at && (
            <> · aufgelöst: {new Date(alert.resolved_at).toLocaleTimeString("de-DE")}</>
          )}
        </div>
      </div>

      <style>{`
        .alert-row {
          display: flex; align-items: flex-start; gap: 0.75rem;
          background: var(--surface); border: 0.5px solid var(--border);
          border-radius: 8px; padding: 0.75rem 1rem;
          transition: border-color 0.15s;
        }
        .alert-row:hover { border-color: var(--brand); }
        .alert-row--resolved { opacity: 0.6; }
        .alert-row__icon { flex-shrink: 0; margin-top: 2px; }
        .alert-row__content { flex: 1; }
        .alert-row__title { display: flex; align-items: center; gap: 0.5rem; margin-bottom: 0.25rem; }
        .alert-row__type { font-size: 0.8125rem; font-weight: 700; font-family: "JetBrains Mono", monospace; }
        .alert-row__severity {
          font-size: 0.625rem; font-weight: 600; text-transform: uppercase;
          letter-spacing: 0.05em; color: var(--muted); padding: 0.125rem 0.375rem;
          background: var(--bg); border-radius: 3px; border: 0.5px solid var(--border-sub);
        }
        .alert-row__resolved-tag {
          font-size: 0.625rem; font-weight: 600; color: var(--green);
          background: color-mix(in srgb, var(--green) 12%, transparent);
          padding: 0.125rem 0.375rem; border-radius: 3px;
        }
        .alert-row__meta { font-size: 0.6875rem; color: var(--dim); font-family: "JetBrains Mono", monospace; }
      `}</style>
    </div>
  );
}
