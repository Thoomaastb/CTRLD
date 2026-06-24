"use client";

import { useState } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import {
  Play, Square, RotateCcw, Settings2, ChevronDown,
  ChevronRight, Shield, AlertTriangle, X, Search
} from "lucide-react";
import { apiClient } from "@/lib/api/client";
import { useRole } from "@/lib/hooks/use-auth";
import { addToast } from "@/components/alerts/toast";
import { cn } from "@/lib/utils/cn";

interface Service {
  name: string;
  description: string;
  load_state: string;
  active_state: string;
  sub_state: string;
  enabled: boolean;
  is_protected: boolean;
}

interface ServiceLogsResponse {
  service: string;
  lines: string[];
  count: number;
}

const STATE_COLORS: Record<string, string> = {
  active:      "var(--green)",
  inactive:    "var(--muted)",
  failed:      "var(--red)",
  activating:  "var(--yellow)",
  deactivating: "var(--yellow)",
};

const STATE_BG: Record<string, string> = {
  active:   "color-mix(in srgb, var(--green) 12%, transparent)",
  inactive: "var(--border-sub)",
  failed:   "color-mix(in srgb, var(--red) 12%, transparent)",
};

export default function ServicesPage() {
  const role = useRole();
  const qc = useQueryClient();
  const [search, setSearch] = useState("");
  const [stateFilter, setStateFilter] = useState("");
  const [selectedService, setSelectedService] = useState<string | null>(null);
  const [confirmAction, setConfirmAction] = useState<{ service: string; action: string } | null>(null);

  const { data, isLoading } = useQuery({
    queryKey: ["services"],
    queryFn: () => apiClient.get<{ services: Service[]; count: number }>("/services"),
    refetchInterval: 30_000,
  });

  const { data: logsData, isLoading: logsLoading } = useQuery({
    queryKey: ["services", selectedService, "logs"],
    queryFn: () => apiClient.get<ServiceLogsResponse>(`/services/${selectedService}/logs?lines=100`),
    enabled: !!selectedService,
    refetchInterval: 10_000,
  });

  const action = useMutation({
    mutationFn: ({ name, act }: { name: string; act: string }) =>
      apiClient.post(`/services/${name}/action`, { action: act }),
    onSuccess: (_, vars) => {
      qc.invalidateQueries({ queryKey: ["services"] });
      addToast({ type: "resolved", title: "Service-Aktion", message: `${vars.act} für ${vars.name} erfolgreich` });
      setConfirmAction(null);
    },
    onError: (err: Error, vars) => {
      addToast({ type: "critical", title: "Fehler", message: err.message || `${vars.act} fehlgeschlagen` });
      setConfirmAction(null);
    },
  });

  function handleAction(name: string, act: string) {
    const needsConfirm = act === "stop" || act === "disable";
    if (needsConfirm) {
      if (confirmAction?.service === name && confirmAction?.action === act) {
        action.mutate({ name, act });
      } else {
        setConfirmAction({ service: name, action: act });
        setTimeout(() => setConfirmAction(null), 3000);
      }
    } else {
      action.mutate({ name, act });
    }
  }

  const services = data?.services ?? [];
  const filtered = services.filter((s) => {
    if (stateFilter && s.active_state !== stateFilter) return false;
    if (search && !s.name.toLowerCase().includes(search.toLowerCase()) &&
        !s.description.toLowerCase().includes(search.toLowerCase())) return false;
    return true;
  });

  const counts = {
    active:   services.filter(s => s.active_state === "active").length,
    inactive: services.filter(s => s.active_state === "inactive").length,
    failed:   services.filter(s => s.active_state === "failed").length,
  };

  return (
    <div className="services-page">
      {/* Header */}
      <div className="services-header">
        <div>
          <h1 className="page-title">Services</h1>
          <p className="page-subtitle">
            {data?.count ?? 0} Services gesamt &nbsp;·&nbsp;
            <span style={{ color: "var(--green)" }}>{counts.active} aktiv</span>
            {counts.failed > 0 && (
              <> &nbsp;·&nbsp; <span style={{ color: "var(--red)" }}>{counts.failed} fehlerhaft</span></>
            )}
          </p>
        </div>
      </div>

      {/* Filter */}
      <div className="services-filters">
        <div className="filter-search">
          <Search size={13} />
          <input
            type="text"
            placeholder="Service suchen…"
            value={search}
            onChange={e => setSearch(e.target.value)}
            className="filter-search__input"
          />
          {search && <button onClick={() => setSearch("")} className="filter-clear"><X size={12} /></button>}
        </div>

        <div className="state-pills">
          {[
            { val: "",         label: "Alle" },
            { val: "active",   label: `Aktiv (${counts.active})` },
            { val: "inactive", label: `Inaktiv (${counts.inactive})` },
            { val: "failed",   label: `Fehler (${counts.failed})` },
          ].map(({ val, label }) => (
            <button
              key={val}
              className={cn("state-pill", stateFilter === val && "state-pill--active")}
              onClick={() => setStateFilter(val)}
            >
              {label}
            </button>
          ))}
        </div>
      </div>

      {/* Service-Liste */}
      <div className="services-list">
        {isLoading && <div className="services-loading">Services werden geladen…</div>}

        {filtered.map((svc) => (
          <div key={svc.name} className="service-row">
            <div className="service-row__main">
              {/* Status-Dot */}
              <div
                className="service-row__dot"
                style={{ background: STATE_COLORS[svc.active_state] ?? "var(--muted)" }}
              />

              {/* Info */}
              <div className="service-row__info">
                <div className="service-row__name">
                  {svc.name}
                  {svc.is_protected && (
                    <span className="service-row__protected" title="Kritischer Service">
                      <Shield size={11} />
                    </span>
                  )}
                </div>
                <div className="service-row__desc">{svc.description || "—"}</div>
              </div>

              {/* Status Badge */}
              <div className="service-row__badges">
                <span
                  className="service-badge"
                  style={{
                    color: STATE_COLORS[svc.active_state],
                    background: STATE_BG[svc.active_state] ?? "var(--border-sub)",
                  }}
                >
                  {svc.sub_state || svc.active_state}
                </span>
                <span className={cn("enabled-badge", svc.enabled ? "enabled-badge--on" : "enabled-badge--off")}>
                  {svc.enabled ? "enabled" : "disabled"}
                </span>
              </div>

              {/* Aktionen */}
              {role === "admin" && (
                <div className="service-row__actions">
                  {svc.active_state !== "active" && (
                    <ActionBtn
                      icon={<Play size={12} />}
                      label="Start"
                      onClick={() => handleAction(svc.name, "start")}
                      loading={action.isPending}
                      color="var(--green)"
                    />
                  )}
                  {svc.active_state === "active" && (
                    <ActionBtn
                      icon={<Square size={12} />}
                      label="Stop"
                      onClick={() => handleAction(svc.name, "stop")}
                      loading={action.isPending}
                      color="var(--red)"
                      disabled={svc.is_protected}
                      confirm={confirmAction?.service === svc.name && confirmAction?.action === "stop"}
                    />
                  )}
                  <ActionBtn
                    icon={<RotateCcw size={12} />}
                    label="Restart"
                    onClick={() => handleAction(svc.name, "restart")}
                    loading={action.isPending}
                    color="var(--yellow)"
                  />
                </div>
              )}

              {/* Logs Toggle */}
              <button
                className="service-row__logs-btn"
                onClick={() => setSelectedService(selectedService === svc.name ? null : svc.name)}
                title="Logs anzeigen"
              >
                {selectedService === svc.name ? <ChevronDown size={14} /> : <ChevronRight size={14} />}
              </button>
            </div>

            {/* Logs Drawer */}
            {selectedService === svc.name && (
              <div className="service-logs">
                {logsLoading ? (
                  <div className="service-logs__loading">Logs werden geladen…</div>
                ) : (
                  <div className="service-logs__content">
                    {(logsData?.lines ?? []).map((line, i) => (
                      <div key={i} className={cn("log-line",
                        line.includes("error") || line.includes("Error") || line.includes("FAILED") ? "log-line--error" :
                        line.includes("warn") || line.includes("Warn") ? "log-line--warn" : ""
                      )}>
                        {line}
                      </div>
                    ))}
                    {logsData?.count === 0 && (
                      <div className="service-logs__empty">Keine Log-Einträge gefunden.</div>
                    )}
                  </div>
                )}
              </div>
            )}
          </div>
        ))}

        {!isLoading && filtered.length === 0 && (
          <div className="services-empty">Keine Services gefunden.</div>
        )}
      </div>

      <style>{`
        .services-page { display: flex; flex-direction: column; gap: 1rem; }

        .services-header { display: flex; align-items: flex-start; justify-content: space-between; }
        .page-title { font-size: 1.125rem; font-weight: 600; color: var(--text); margin: 0 0 0.25rem; }
        .page-subtitle { font-size: 0.75rem; color: var(--muted); margin: 0; }

        .services-filters { display: flex; align-items: center; gap: 0.75rem; flex-wrap: wrap; }

        .filter-search {
          display: flex; align-items: center; gap: 0.375rem;
          background: var(--surface); border: 0.5px solid var(--border);
          border-radius: 6px; padding: 0 0.625rem; color: var(--muted);
          transition: border-color 0.15s;
        }
        .filter-search:focus-within { border-color: var(--brand-l); }
        .filter-search__input {
          background: none; border: none; color: var(--text); font-size: 0.75rem;
          padding: 0.375rem 0; outline: none; width: 200px;
        }
        .filter-search__input::placeholder { color: var(--dim); }
        .filter-clear { background: none; border: none; color: var(--muted); cursor: pointer; display: flex; }

        .state-pills { display: flex; gap: 0.375rem; }
        .state-pill {
          background: var(--surface); border: 0.5px solid var(--border);
          color: var(--muted); font-size: 0.6875rem; font-weight: 500;
          padding: 0.25rem 0.625rem; border-radius: 999px; cursor: pointer; transition: all 0.15s;
        }
        .state-pill--active { background: var(--brand-bg); color: var(--brand-t); border-color: var(--brand); }

        .services-list { display: flex; flex-direction: column; gap: 0.5rem; }
        .services-loading, .services-empty {
          padding: 2rem; text-align: center; color: var(--muted); font-size: 0.8125rem;
          background: var(--surface); border: 0.5px solid var(--border); border-radius: var(--radius);
        }

        .service-row {
          background: var(--surface); border: 0.5px solid var(--border);
          border-radius: var(--radius); overflow: hidden; transition: border-color 0.15s;
        }
        .service-row:hover { border-color: var(--brand); }

        .service-row__main {
          display: flex; align-items: center; gap: 0.75rem; padding: 0.75rem 1rem;
        }

        .service-row__dot {
          width: 8px; height: 8px; border-radius: 50%; flex-shrink: 0;
        }

        .service-row__info { flex: 1; min-width: 0; }
        .service-row__name {
          font-size: 0.8125rem; font-weight: 600; color: var(--text);
          font-family: "JetBrains Mono", monospace; display: flex; align-items: center; gap: 0.375rem;
        }
        .service-row__protected { color: var(--yellow); display: flex; }
        .service-row__desc { font-size: 0.6875rem; color: var(--muted); margin-top: 0.125rem; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }

        .service-row__badges { display: flex; gap: 0.375rem; align-items: center; flex-shrink: 0; }
        .service-badge {
          font-size: 0.625rem; font-weight: 600; padding: 0.125rem 0.5rem;
          border-radius: 4px; font-family: "JetBrains Mono", monospace;
        }
        .enabled-badge {
          font-size: 0.625rem; padding: 0.125rem 0.375rem; border-radius: 4px; font-weight: 500;
        }
        .enabled-badge--on { color: var(--green); background: color-mix(in srgb, var(--green) 10%, transparent); }
        .enabled-badge--off { color: var(--dim); background: var(--border-sub); }

        .service-row__actions { display: flex; gap: 0.25rem; flex-shrink: 0; }

        .service-row__logs-btn {
          background: none; border: none; color: var(--muted); cursor: pointer;
          display: flex; align-items: center; padding: 0.25rem; border-radius: 4px;
          transition: color 0.15s, background 0.15s;
        }
        .service-row__logs-btn:hover { color: var(--text); background: var(--border-sub); }

        /* Logs Drawer */
        .service-logs {
          border-top: 0.5px solid var(--border-sub); background: var(--bg);
          padding: 0.75rem 1rem; max-height: 300px; overflow-y: auto;
        }
        .service-logs__loading, .service-logs__empty {
          color: var(--muted); font-size: 0.75rem; text-align: center; padding: 0.5rem;
        }
        .service-logs__content { font-family: "JetBrains Mono", monospace; font-size: 0.6875rem; }
        .log-line {
          color: var(--muted); padding: 0.125rem 0; white-space: nowrap;
          overflow: hidden; text-overflow: ellipsis; border-bottom: 0.5px solid transparent;
        }
        .log-line:hover { overflow: visible; white-space: normal; }
        .log-line--error { color: var(--red); }
        .log-line--warn { color: var(--yellow); }
      `}</style>
    </div>
  );
}

interface ActionBtnProps {
  icon: React.ReactNode;
  label: string;
  onClick: () => void;
  loading?: boolean;
  color?: string;
  disabled?: boolean;
  confirm?: boolean;
}

function ActionBtn({ icon, label, onClick, loading, color, disabled, confirm }: ActionBtnProps) {
  return (
    <button
      className={cn("action-btn", confirm && "action-btn--confirm")}
      onClick={onClick}
      disabled={loading || disabled}
      title={disabled ? "Geschützter Service" : confirm ? "Nochmals klicken zum Bestätigen" : label}
      style={{ "--btn-color": color ?? "var(--muted)" } as React.CSSProperties}
    >
      {icon}

      <style>{`
        .action-btn {
          display: flex; align-items: center; justify-content: center;
          width: 28px; height: 28px; border-radius: 6px;
          background: none; border: 0.5px solid transparent;
          color: var(--muted); cursor: pointer; transition: all 0.15s;
        }
        .action-btn:hover:not(:disabled) {
          color: var(--btn-color, var(--muted));
          border-color: color-mix(in srgb, var(--btn-color, var(--muted)) 40%, transparent);
          background: color-mix(in srgb, var(--btn-color, var(--muted)) 10%, transparent);
        }
        .action-btn:disabled { opacity: 0.3; cursor: not-allowed; }
        .action-btn--confirm {
          color: var(--red) !important;
          border-color: var(--red) !important;
          background: color-mix(in srgb, var(--red) 15%, transparent) !important;
          animation: confirm-shake 0.3s ease;
        }
        @keyframes confirm-shake { 0%,100%{transform:rotate(0)} 25%{transform:rotate(-5deg)} 75%{transform:rotate(5deg)} }
      `}</style>
    </button>
  );
}
