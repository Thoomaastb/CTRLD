"use client";

import { useState } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import {
  Shield, RefreshCw, AlertTriangle, CheckCircle,
  ExternalLink, Server, Lock, Unlock
} from "lucide-react";
import { apiClient } from "@/lib/api/client";
import { useRole } from "@/lib/hooks/use-auth";
import { addToast } from "@/components/alerts/toast";
import { cn } from "@/lib/utils/cn";

interface UpdateStatus {
  current_version: string;
  latest_version: string;
  has_update: boolean;
  last_checked: string;
  release?: {
    version: string;
    url: string;
    published_at: string;
    body: string;
    is_prerelease: boolean;
  };
}

interface TLSInfo {
  has_certificate: boolean;
  cert?: {
    subject: string;
    issuer: string;
    not_before: string;
    not_after: string;
    dns_names: string[];
    is_self_signed: boolean;
    days_left: number;
    is_expired: boolean;
    is_valid: boolean;
  };
}

export default function SettingsPage() {
  const role = useRole();
  const qc = useQueryClient();
  const isAdmin = role === "admin";

  const { data: updateData } = useQuery<UpdateStatus>({
    queryKey: ["system", "update-status"],
    queryFn: () => apiClient.get<UpdateStatus>("/system/update-status"),
    enabled: isAdmin,
    staleTime: 60_000,
  });

  const { data: tlsData } = useQuery<TLSInfo>({
    queryKey: ["system", "tls"],
    queryFn: () => apiClient.get<TLSInfo>("/system/tls"),
    enabled: isAdmin,
  });

  // TLS-Generierung
  const [tlsHosts, setTlsHosts] = useState("");
  const [tlsDays, setTlsDays] = useState(365);

  const generateTLS = useMutation({
    mutationFn: () => apiClient.post("/system/tls/generate", {
      hosts: tlsHosts.split(",").map(h => h.trim()).filter(Boolean),
      valid_days: tlsDays,
    }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["system", "tls"] });
      addToast({ type: "resolved", title: "TLS", message: "Zertifikat erfolgreich generiert. Neustart erforderlich." });
    },
    onError: () => {
      addToast({ type: "critical", title: "TLS Fehler", message: "Zertifikat konnte nicht generiert werden." });
    },
  });

  return (
    <div className="settings-page">
      <div className="settings-header">
        <h1 className="page-title">Einstellungen</h1>
        <p className="page-subtitle">System-Konfiguration und Wartung</p>
      </div>

      {/* Update-Status */}
      {isAdmin && (
        <section className="settings-section">
          <div className="section-header">
            <RefreshCw size={15} />
            <h2 className="section-title">Software-Updates</h2>
          </div>

          {updateData ? (
            <div className="update-card">
              <div className="version-row">
                <div className="version-item">
                  <div className="version-label">Installiert</div>
                  <div className="version-value">{updateData.current_version}</div>
                </div>
                <div className="version-item">
                  <div className="version-label">Verfügbar</div>
                  <div className={cn("version-value", updateData.has_update && "version-value--new")}>
                    {updateData.latest_version || "—"}
                  </div>
                </div>
                <div className="version-item">
                  <div className="version-label">Geprüft</div>
                  <div className="version-value version-value--muted">
                    {updateData.last_checked
                      ? new Date(updateData.last_checked).toLocaleString("de-DE")
                      : "—"}
                  </div>
                </div>
              </div>

              {updateData.has_update && updateData.release && (
                <div className="update-available">
                  <AlertTriangle size={14} className="update-available__icon" />
                  <div className="update-available__content">
                    <div className="update-available__title">
                      Neue Version verfügbar: {updateData.release.version}
                    </div>
                    <div className="update-available__desc">
                      Veröffentlicht: {new Date(updateData.release.published_at).toLocaleDateString("de-DE")}
                    </div>
                  </div>
                  <a
                    href={updateData.release.url}
                    target="_blank"
                    rel="noopener noreferrer"
                    className="update-btn"
                  >
                    Release-Notes <ExternalLink size={12} />
                  </a>
                </div>
              )}

              {!updateData.has_update && updateData.latest_version && (
                <div className="up-to-date">
                  <CheckCircle size={14} />
                  CTRLD ist aktuell
                </div>
              )}
            </div>
          ) : (
            <div className="settings-placeholder">Update-Status wird geladen…</div>
          )}
        </section>
      )}

      {/* TLS-Konfiguration */}
      {isAdmin && (
        <section className="settings-section">
          <div className="section-header">
            <Shield size={15} />
            <h2 className="section-title">TLS-Zertifikat</h2>
          </div>

          {tlsData?.has_certificate && tlsData.cert ? (
            <div className="tls-card">
              <div className="tls-status">
                {tlsData.cert.is_valid ? (
                  <span className="tls-badge tls-badge--valid"><Lock size={12} /> Gültig</span>
                ) : (
                  <span className="tls-badge tls-badge--expired"><Unlock size={12} /> Abgelaufen</span>
                )}
                {tlsData.cert.is_self_signed && (
                  <span className="tls-badge tls-badge--self">Self-Signed</span>
                )}
              </div>

              <div className="tls-details">
                <div className="tls-row">
                  <span className="tls-label">Subject</span>
                  <span className="tls-value">{tlsData.cert.subject}</span>
                </div>
                <div className="tls-row">
                  <span className="tls-label">Gültig bis</span>
                  <span className={cn("tls-value", tlsData.cert.days_left < 30 && "tls-value--warn")}>
                    {new Date(tlsData.cert.not_after).toLocaleDateString("de-DE")}
                    {" "}({tlsData.cert.days_left} Tage)
                  </span>
                </div>
                {tlsData.cert.dns_names.length > 0 && (
                  <div className="tls-row">
                    <span className="tls-label">DNS-Namen</span>
                    <span className="tls-value">{tlsData.cert.dns_names.join(", ")}</span>
                  </div>
                )}
              </div>
            </div>
          ) : (
            <div className="tls-empty">
              <Unlock size={20} />
              <p>Kein TLS-Zertifikat konfiguriert. CTRLD läuft ohne HTTPS.</p>
            </div>
          )}

          {/* Self-Signed Generator */}
          <div className="tls-generator">
            <h3 className="tls-generator__title">Self-Signed Zertifikat generieren</h3>
            <p className="tls-generator__desc">
              Für interne/Lab-Deployments. Für Produktion empfehlen wir Let's Encrypt (v2.x).
            </p>

            <div className="tls-form">
              <div className="tls-field">
                <label className="tls-field__label">Hostnames / IPs <span className="tls-field__hint">(kommagetrennt)</span></label>
                <input
                  type="text"
                  value={tlsHosts}
                  onChange={e => setTlsHosts(e.target.value)}
                  placeholder="example.com, 192.168.1.100, localhost"
                  className="tls-input"
                />
              </div>
              <div className="tls-field tls-field--narrow">
                <label className="tls-field__label">Gültigkeitsdauer</label>
                <div className="tls-days">
                  <input
                    type="number"
                    value={tlsDays}
                    onChange={e => setTlsDays(Number(e.target.value))}
                    min={30} max={3650}
                    className="tls-input"
                  />
                  <span className="tls-days__label">Tage</span>
                </div>
              </div>
            </div>

            <button
              className="tls-generate-btn"
              onClick={() => generateTLS.mutate()}
              disabled={generateTLS.isPending || !tlsHosts.trim()}
            >
              {generateTLS.isPending ? "Generiere…" : "Zertifikat generieren"}
            </button>

            {generateTLS.isSuccess && (
              <div className="tls-restart-warning">
                ⚠️ CTRLD muss neu gestartet werden damit TLS aktiv wird:
                <code>sudo systemctl restart ctrld</code>
              </div>
            )}
          </div>
        </section>
      )}

      {/* Installation-Info */}
      <section className="settings-section">
        <div className="section-header">
          <Server size={15} />
          <h2 className="section-title">Installation</h2>
        </div>

        <div className="install-info">
          <p className="install-info__desc">
            CTRLD auf einem neuen Server installieren:
          </p>
          <code className="install-command">
            curl -fsSL https://get.ctrld.io | sudo bash
          </code>
        </div>
      </section>

      <style>{`
        .settings-page { display: flex; flex-direction: column; gap: 1.5rem; }

        .settings-header { margin-bottom: 0.5rem; }
        .page-title { font-size: 1.125rem; font-weight: 600; color: var(--text); margin: 0 0 0.25rem; }
        .page-subtitle { font-size: 0.75rem; color: var(--muted); margin: 0; }

        .settings-section {
          background: var(--surface); border: 0.5px solid var(--border);
          border-radius: var(--radius); padding: 1.25rem;
          display: flex; flex-direction: column; gap: 1rem;
        }

        .section-header {
          display: flex; align-items: center; gap: 0.5rem;
          color: var(--muted);
        }
        .section-title {
          font-size: 0.75rem; font-weight: 600; text-transform: uppercase;
          letter-spacing: 0.07em; margin: 0; color: var(--muted);
        }

        /* Update */
        .update-card { display: flex; flex-direction: column; gap: 0.875rem; }

        .version-row { display: flex; gap: 2rem; }
        .version-item { display: flex; flex-direction: column; gap: 0.25rem; }
        .version-label { font-size: 0.6875rem; color: var(--dim); text-transform: uppercase; letter-spacing: 0.05em; }
        .version-value { font-family: "JetBrains Mono", monospace; font-size: 0.875rem; font-weight: 600; color: var(--text); }
        .version-value--new { color: var(--green); }
        .version-value--muted { font-size: 0.75rem; color: var(--muted); font-weight: 400; }

        .update-available {
          display: flex; align-items: center; gap: 0.75rem;
          background: color-mix(in srgb, var(--yellow) 8%, transparent);
          border: 0.5px solid color-mix(in srgb, var(--yellow) 25%, transparent);
          border-radius: 8px; padding: 0.75rem 1rem;
        }
        .update-available__icon { color: var(--yellow); flex-shrink: 0; }
        .update-available__content { flex: 1; }
        .update-available__title { font-size: 0.8125rem; font-weight: 600; color: var(--text); }
        .update-available__desc { font-size: 0.75rem; color: var(--muted); }

        .update-btn {
          display: flex; align-items: center; gap: 0.375rem;
          background: var(--brand); color: white; border: none;
          font-size: 0.75rem; font-weight: 600; padding: 0.375rem 0.75rem;
          border-radius: 6px; cursor: pointer; text-decoration: none;
          transition: background 0.15s; white-space: nowrap;
        }
        .update-btn:hover { background: var(--brand-l); }

        .up-to-date {
          display: flex; align-items: center; gap: 0.5rem;
          color: var(--green); font-size: 0.8125rem; font-weight: 500;
        }

        /* TLS */
        .tls-card { display: flex; flex-direction: column; gap: 0.875rem; }
        .tls-status { display: flex; gap: 0.5rem; }
        .tls-badge {
          display: inline-flex; align-items: center; gap: 0.25rem;
          font-size: 0.6875rem; font-weight: 600; padding: 0.25rem 0.625rem;
          border-radius: 4px;
        }
        .tls-badge--valid { color: var(--green); background: color-mix(in srgb, var(--green) 12%, transparent); }
        .tls-badge--expired { color: var(--red); background: color-mix(in srgb, var(--red) 12%, transparent); }
        .tls-badge--self { color: var(--muted); background: var(--border-sub); }

        .tls-details { display: flex; flex-direction: column; gap: 0.375rem; }
        .tls-row { display: flex; gap: 1rem; font-size: 0.75rem; }
        .tls-label { color: var(--muted); width: 80px; flex-shrink: 0; }
        .tls-value { color: var(--text); font-family: "JetBrains Mono", monospace; }
        .tls-value--warn { color: var(--yellow); }

        .tls-empty {
          display: flex; flex-direction: column; align-items: center; gap: 0.5rem;
          padding: 1.5rem; color: var(--dim); text-align: center;
        }
        .tls-empty p { font-size: 0.8125rem; margin: 0; }

        .tls-generator {
          border-top: 0.5px solid var(--border-sub); padding-top: 1rem;
          display: flex; flex-direction: column; gap: 0.875rem;
        }
        .tls-generator__title { font-size: 0.875rem; font-weight: 600; color: var(--text); margin: 0; }
        .tls-generator__desc { font-size: 0.75rem; color: var(--muted); margin: 0; }

        .tls-form { display: flex; gap: 0.75rem; flex-wrap: wrap; }
        .tls-field { display: flex; flex-direction: column; gap: 0.375rem; flex: 1; min-width: 200px; }
        .tls-field--narrow { flex: 0 0 140px; min-width: unset; }
        .tls-field__label { font-size: 0.75rem; color: var(--muted); font-weight: 500; }
        .tls-field__hint { font-weight: 400; color: var(--dim); }
        .tls-input {
          background: var(--bg); border: 0.5px solid var(--border); color: var(--text);
          font-size: 0.8125rem; padding: 0.5rem 0.75rem; border-radius: 6px;
          outline: none; transition: border-color 0.15s; width: 100%;
        }
        .tls-input:focus { border-color: var(--brand-l); }
        .tls-days { display: flex; align-items: center; gap: 0.5rem; }
        .tls-days__label { font-size: 0.75rem; color: var(--muted); white-space: nowrap; }

        .tls-generate-btn {
          background: var(--brand); color: white; border: none;
          font-size: 0.875rem; font-weight: 600; padding: 0.5rem 1.25rem;
          border-radius: 6px; cursor: pointer; transition: background 0.15s;
          align-self: flex-start;
        }
        .tls-generate-btn:hover:not(:disabled) { background: var(--brand-l); }
        .tls-generate-btn:disabled { opacity: 0.5; cursor: not-allowed; }

        .tls-restart-warning {
          background: color-mix(in srgb, var(--yellow) 8%, transparent);
          border: 0.5px solid color-mix(in srgb, var(--yellow) 25%, transparent);
          border-radius: 6px; padding: 0.75rem 1rem; font-size: 0.75rem; color: var(--yellow);
          display: flex; flex-direction: column; gap: 0.375rem;
        }
        .tls-restart-warning code {
          font-family: "JetBrains Mono", monospace; font-size: 0.8125rem;
          color: var(--text); background: var(--bg);
          padding: 0.25rem 0.5rem; border-radius: 4px;
          display: inline-block;
        }

        /* Install-Info */
        .install-info { display: flex; flex-direction: column; gap: 0.75rem; }
        .install-info__desc { font-size: 0.8125rem; color: var(--muted); margin: 0; }
        .install-command {
          font-family: "JetBrains Mono", monospace; font-size: 0.875rem;
          color: var(--green); background: var(--bg);
          border: 0.5px solid var(--border); border-radius: 6px;
          padding: 0.75rem 1rem; display: block; user-select: all;
        }

        .settings-placeholder { color: var(--muted); font-size: 0.8125rem; }
      `}</style>
    </div>
  );
}
