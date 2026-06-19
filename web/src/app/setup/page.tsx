"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import { authApi } from "@/lib/api/auth";
import { cn } from "@/lib/utils/cn";

type Step = "admin" | "mfa" | "done";

export default function SetupPage() {
  const [step, setStep] = useState<Step>("admin");
  const [adminId, setAdminId] = useState("");
  const [totpSetup, setTotpSetup] = useState<{ secret: string; qr_code: string; manual_entry_key: string } | null>(null);
  const [backupCodes, setBackupCodes] = useState<string[]>([]);
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(false);

  // Schritt 1: Admin erstellen
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [passwordConfirm, setPasswordConfirm] = useState("");

  // Schritt 2: MFA
  const [totpCode, setTotpCode] = useState("");

  const router = useRouter();

  async function handleCreateAdmin(e: React.FormEvent) {
    e.preventDefault();
    setError("");

    if (password !== passwordConfirm) {
      setError("Passwörter stimmen nicht überein.");
      return;
    }
    if (password.length < 12) {
      setError("Passwort muss mindestens 12 Zeichen haben.");
      return;
    }

    setLoading(true);
    try {
      const result = await authApi.createAdmin(email, password);
      setAdminId(result.user_id);

      // TOTP-Setup initiieren
      // Erst einloggen um JWT zu bekommen, dann TOTP initiieren
      // Für Setup-Flow: direkt TOTP-Daten generieren (kein Auth-Gate im Setup)
      const totp = await authApi.initiateTOTP();
      setTotpSetup(totp);
      setStep("mfa");
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : "Fehler beim Erstellen des Admin-Accounts.";
      setError(msg);
    } finally {
      setLoading(false);
    }
  }

  async function handleConfirmMFA(e: React.FormEvent) {
    e.preventDefault();
    if (!totpSetup) return;
    setError("");
    setLoading(true);

    try {
      const result = await authApi.confirmTOTP(totpSetup.secret, totpCode, "Authenticator App");
      setBackupCodes(result.backup_codes);

      // Setup abschließen
      await authApi.completeSetup(adminId);
      setStep("done");
    } catch {
      setError("Ungültiger Code. Bitte erneut versuchen.");
    } finally {
      setLoading(false);
    }
  }

  return (
    <div className="setup-root">
      <div className="setup-card">
        {/* Header */}
        <div className="setup-header">
          <svg width="28" height="28" viewBox="0 0 32 32" fill="none">
            <rect width="32" height="32" rx="8" fill="var(--brand)" />
            <path d="M16 6L26 11V21L16 26L6 21V11L16 6Z" stroke="white" strokeWidth="2" strokeLinejoin="round" />
            <circle cx="16" cy="16" r="3" fill="white" />
          </svg>
          <span className="setup-logo-text">CTRLD</span>
        </div>

        {/* Steps */}
        <div className="steps">
          {(["admin", "mfa", "done"] as Step[]).map((s, i) => (
            <div key={s} className={cn("step", step === s && "step--active", stepIndex(step) > i && "step--done")}>
              <div className="step-dot">
                {stepIndex(step) > i ? "✓" : i + 1}
              </div>
              <span className="step-label">
                {s === "admin" ? "Admin" : s === "mfa" ? "2FA" : "Fertig"}
              </span>
            </div>
          ))}
        </div>

        {error && (
          <div className="setup-error" role="alert">{error}</div>
        )}

        {/* ── Schritt 1: Admin ── */}
        {step === "admin" && (
          <form onSubmit={handleCreateAdmin} className="setup-form">
            <h2 className="setup-step-title">Admin-Account erstellen</h2>
            <p className="setup-step-desc">Dieser Account hat vollen Zugriff auf CTRLD.</p>

            <div className="field">
              <label className="field-label">E-Mail</label>
              <input type="email" required value={email} onChange={e => setEmail(e.target.value)}
                className="field-input" placeholder="admin@example.com" disabled={loading} />
            </div>
            <div className="field">
              <label className="field-label">Passwort <span className="field-hint">(min. 12 Zeichen)</span></label>
              <input type="password" required value={password} onChange={e => setPassword(e.target.value)}
                className="field-input" placeholder="••••••••••••" disabled={loading} />
            </div>
            <div className="field">
              <label className="field-label">Passwort bestätigen</label>
              <input type="password" required value={passwordConfirm} onChange={e => setPasswordConfirm(e.target.value)}
                className="field-input" placeholder="••••••••••••" disabled={loading} />
            </div>
            <button type="submit" disabled={loading} className="setup-btn">
              {loading ? "Erstelle Account…" : "Weiter →"}
            </button>
          </form>
        )}

        {/* ── Schritt 2: MFA ── */}
        {step === "mfa" && totpSetup && (
          <form onSubmit={handleConfirmMFA} className="setup-form">
            <h2 className="setup-step-title">Authenticator einrichten</h2>
            <p className="setup-step-desc">Scanne den QR-Code mit einer Authenticator-App (Google Authenticator, Authy, etc.)</p>

            {/* QR-Code */}
            <div className="qr-wrapper">
              {/* eslint-disable-next-line @next/next/no-img-element */}
              <img src={totpSetup.qr_code} alt="TOTP QR-Code" width={200} height={200} className="qr-img" />
            </div>

            {/* Manueller Key */}
            <div className="manual-key">
              <span className="manual-key__label">Manueller Key:</span>
              <code className="manual-key__value">{totpSetup.manual_entry_key}</code>
            </div>

            <div className="field" style={{ marginTop: "1rem" }}>
              <label className="field-label">Bestätigungscode</label>
              <input
                type="text"
                inputMode="numeric"
                pattern="[0-9]*"
                maxLength={6}
                value={totpCode}
                onChange={e => setTotpCode(e.target.value.replace(/\D/g, "").slice(0, 6))}
                className="field-input field-input--mono field-input--center"
                placeholder="000000"
                disabled={loading}
              />
            </div>
            <button type="submit" disabled={loading || totpCode.length < 6} className="setup-btn">
              {loading ? "Prüfe…" : "Einrichten →"}
            </button>
          </form>
        )}

        {/* ── Schritt 3: Fertig ── */}
        {step === "done" && (
          <div className="setup-done">
            <div className="done-icon">✓</div>
            <h2 className="setup-step-title">Setup abgeschlossen!</h2>
            <p className="setup-step-desc">CTRLD ist einsatzbereit.</p>

            {backupCodes.length > 0 && (
              <div className="backup-codes">
                <p className="backup-codes__warning">
                  ⚠️ Speichere diese Backup-Codes sicher — sie werden nicht erneut angezeigt!
                </p>
                <div className="backup-codes__list">
                  {backupCodes.map((code) => (
                    <code key={code} className="backup-code">{code}</code>
                  ))}
                </div>
              </div>
            )}

            <button onClick={() => router.push("/login")} className="setup-btn" style={{ marginTop: "1.5rem" }}>
              Zum Login →
            </button>
          </div>
        )}
      </div>

      <style>{`
        .setup-root {
          min-height: 100dvh;
          display: flex;
          align-items: center;
          justify-content: center;
          background: var(--bg);
          padding: 1rem;
        }
        .setup-card {
          width: 100%;
          max-width: 440px;
          background: var(--surface);
          border: 0.5px solid var(--border);
          border-radius: var(--radius);
          padding: 2rem;
        }
        .setup-header {
          display: flex;
          align-items: center;
          gap: 0.625rem;
          margin-bottom: 1.75rem;
        }
        .setup-logo-text {
          font-size: 1.125rem;
          font-weight: 700;
          letter-spacing: 0.05em;
          color: var(--text);
        }
        .steps {
          display: flex;
          align-items: center;
          gap: 0;
          margin-bottom: 1.75rem;
        }
        .step {
          display: flex;
          align-items: center;
          gap: 0.5rem;
          flex: 1;
          position: relative;
        }
        .step:not(:last-child)::after {
          content: '';
          position: absolute;
          right: 0;
          top: 50%;
          transform: translateY(-50%);
          width: calc(100% - 2rem);
          height: 1px;
          background: var(--border);
          left: 2rem;
        }
        .step-dot {
          width: 24px;
          height: 24px;
          border-radius: 50%;
          background: var(--bg);
          border: 1.5px solid var(--border);
          color: var(--muted);
          font-size: 0.75rem;
          font-weight: 600;
          display: flex;
          align-items: center;
          justify-content: center;
          flex-shrink: 0;
          z-index: 1;
        }
        .step--active .step-dot {
          background: var(--brand);
          border-color: var(--brand);
          color: white;
        }
        .step--done .step-dot {
          background: var(--green);
          border-color: var(--green);
          color: white;
        }
        .step-label { font-size: 0.75rem; color: var(--muted); }
        .step--active .step-label { color: var(--text); font-weight: 500; }
        .setup-error {
          background: color-mix(in srgb, var(--red) 12%, transparent);
          border: 0.5px solid color-mix(in srgb, var(--red) 40%, transparent);
          color: var(--red);
          border-radius: 6px;
          padding: 0.625rem 0.875rem;
          font-size: 0.8125rem;
          margin-bottom: 1.25rem;
        }
        .setup-form { display: flex; flex-direction: column; gap: 1rem; }
        .setup-step-title {
          font-size: 1.125rem;
          font-weight: 600;
          color: var(--text);
          margin: 0 0 0.25rem;
        }
        .setup-step-desc { font-size: 0.8125rem; color: var(--muted); margin: 0 0 0.5rem; }
        .field { display: flex; flex-direction: column; gap: 0.375rem; }
        .field-label { font-size: 0.8125rem; color: var(--muted); font-weight: 500; }
        .field-hint { font-weight: 400; color: var(--dim); }
        .field-input {
          background: var(--bg);
          border: 0.5px solid var(--border);
          border-radius: 6px;
          color: var(--text);
          font-size: 0.875rem;
          padding: 0.5625rem 0.75rem;
          outline: none;
          transition: border-color 0.15s;
          width: 100%;
        }
        .field-input:focus { border-color: var(--brand-l); }
        .field-input--mono { font-family: "JetBrains Mono", monospace; font-size: 1.25rem; letter-spacing: 0.2em; }
        .field-input--center { text-align: center; }
        .setup-btn {
          background: var(--brand);
          color: white;
          border: none;
          border-radius: 6px;
          font-size: 0.875rem;
          font-weight: 600;
          padding: 0.625rem;
          cursor: pointer;
          transition: background 0.15s, opacity 0.15s;
          width: 100%;
        }
        .setup-btn:hover:not(:disabled) { background: var(--brand-l); }
        .setup-btn:disabled { opacity: 0.5; cursor: not-allowed; }
        .qr-wrapper {
          display: flex;
          justify-content: center;
          background: white;
          border-radius: 8px;
          padding: 1rem;
        }
        .qr-img { display: block; }
        .manual-key {
          background: var(--bg);
          border: 0.5px solid var(--border);
          border-radius: 6px;
          padding: 0.625rem 0.875rem;
          font-size: 0.8125rem;
          display: flex;
          flex-direction: column;
          gap: 0.25rem;
        }
        .manual-key__label { color: var(--muted); font-size: 0.75rem; }
        .manual-key__value {
          font-family: "JetBrains Mono", monospace;
          color: var(--text);
          word-break: break-all;
          letter-spacing: 0.05em;
        }
        .setup-done { text-align: center; }
        .done-icon {
          width: 56px;
          height: 56px;
          border-radius: 50%;
          background: color-mix(in srgb, var(--green) 15%, transparent);
          color: var(--green);
          font-size: 1.5rem;
          display: flex;
          align-items: center;
          justify-content: center;
          margin: 0 auto 1rem;
        }
        .backup-codes {
          background: color-mix(in srgb, var(--yellow) 8%, transparent);
          border: 0.5px solid color-mix(in srgb, var(--yellow) 30%, transparent);
          border-radius: 6px;
          padding: 1rem;
          margin-top: 1rem;
          text-align: left;
        }
        .backup-codes__warning {
          font-size: 0.8125rem;
          color: var(--yellow);
          margin: 0 0 0.875rem;
          font-weight: 500;
        }
        .backup-codes__list {
          display: grid;
          grid-template-columns: 1fr 1fr;
          gap: 0.375rem;
        }
        .backup-code {
          font-family: "JetBrains Mono", monospace;
          font-size: 0.8125rem;
          color: var(--text);
          background: var(--surface);
          border: 0.5px solid var(--border);
          border-radius: 4px;
          padding: 0.25rem 0.5rem;
          text-align: center;
        }
      `}</style>
    </div>
  );
}

function stepIndex(step: Step): number {
  return { admin: 0, mfa: 1, done: 2 }[step];
}
