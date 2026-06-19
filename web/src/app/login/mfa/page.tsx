"use client";

import { useState, useRef, useEffect } from "react";
import { useMFAVerify } from "@/lib/hooks/use-auth";

export default function MFAPage() {
  const [code, setCode] = useState("");
  const [useBackup, setUseBackup] = useState(false);
  const inputRef = useRef<HTMLInputElement>(null);
  const verify = useMFAVerify();

  useEffect(() => {
    inputRef.current?.focus();
  }, [useBackup]);

  function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    verify.mutate({ code, type: useBackup ? "backup_code" : "totp" });
  }

  // Code auto-submit bei 6 Ziffern (TOTP)
  function handleCodeChange(val: string) {
    const clean = val.replace(/\D/g, "").slice(0, 6);
    setCode(clean);
    if (clean.length === 6 && !useBackup) {
      verify.mutate({ code: clean, type: "totp" });
    }
  }

  return (
    <div className="mfa-root">
      <div className="mfa-card">
        <div className="mfa-header">
          <div className="mfa-icon">
            <svg width="40" height="40" viewBox="0 0 40 40" fill="none">
              <rect width="40" height="40" rx="10" fill="var(--brand-bg)" />
              <path
                d="M20 10L28 14V22C28 26.4 24.4 30 20 30C15.6 30 12 26.4 12 22V14L20 10Z"
                stroke="var(--brand-l)"
                strokeWidth="2"
                strokeLinejoin="round"
              />
              <path
                d="M17 20L19 22L23 18"
                stroke="var(--brand-l)"
                strokeWidth="2"
                strokeLinecap="round"
                strokeLinejoin="round"
              />
            </svg>
          </div>
          <h1 className="mfa-title">Zwei-Faktor-Authentifizierung</h1>
          <p className="mfa-subtitle">
            {useBackup
              ? "Gib einen Backup-Code ein"
              : "Gib den 6-stelligen Code aus deiner Authenticator-App ein"}
          </p>
        </div>

        {verify.isError && (
          <div className="mfa-error" role="alert">
            Ungültiger Code. Bitte erneut versuchen.
          </div>
        )}

        <form onSubmit={handleSubmit} className="mfa-form">
          {useBackup ? (
            <input
              ref={inputRef}
              type="text"
              value={code}
              onChange={(e) => setCode(e.target.value.toUpperCase())}
              placeholder="XXXXX-XXXXX"
              className="mfa-input mfa-input--backup"
              autoComplete="off"
              disabled={verify.isPending}
            />
          ) : (
            <input
              ref={inputRef}
              type="text"
              inputMode="numeric"
              pattern="[0-9]*"
              maxLength={6}
              value={code}
              onChange={(e) => handleCodeChange(e.target.value)}
              placeholder="000000"
              className="mfa-input mfa-input--totp"
              autoComplete="one-time-code"
              disabled={verify.isPending}
            />
          )}

          <button
            type="submit"
            disabled={verify.isPending || code.length < (useBackup ? 11 : 6)}
            className="mfa-btn"
          >
            {verify.isPending ? "Prüfe…" : "Bestätigen"}
          </button>
        </form>

        <button
          className="mfa-switch"
          onClick={() => { setCode(""); setUseBackup(!useBackup); }}
        >
          {useBackup ? "← Zurück zu TOTP" : "Backup-Code verwenden"}
        </button>
      </div>

      <style>{`
        .mfa-root {
          min-height: 100dvh;
          display: flex;
          align-items: center;
          justify-content: center;
          background: var(--bg);
          padding: 1rem;
        }
        .mfa-card {
          width: 100%;
          max-width: 360px;
          background: var(--surface);
          border: 0.5px solid var(--border);
          border-radius: var(--radius);
          padding: 2rem;
          text-align: center;
        }
        .mfa-header { margin-bottom: 1.5rem; }
        .mfa-icon { display: inline-flex; margin-bottom: 1rem; }
        .mfa-title {
          font-size: 1.125rem;
          font-weight: 600;
          color: var(--text);
          margin: 0 0 0.5rem;
        }
        .mfa-subtitle { font-size: 0.8125rem; color: var(--muted); margin: 0; }
        .mfa-error {
          background: color-mix(in srgb, var(--red) 12%, transparent);
          border: 0.5px solid color-mix(in srgb, var(--red) 40%, transparent);
          color: var(--red);
          border-radius: 6px;
          padding: 0.625rem;
          font-size: 0.8125rem;
          margin-bottom: 1rem;
        }
        .mfa-form { display: flex; flex-direction: column; gap: 0.875rem; }
        .mfa-input {
          background: var(--bg);
          border: 0.5px solid var(--border);
          border-radius: 6px;
          color: var(--text);
          outline: none;
          text-align: center;
          width: 100%;
          transition: border-color 0.15s;
        }
        .mfa-input:focus { border-color: var(--brand-l); }
        .mfa-input--totp {
          font-family: "JetBrains Mono", monospace;
          font-size: 1.75rem;
          font-weight: 600;
          letter-spacing: 0.25em;
          padding: 0.75rem;
        }
        .mfa-input--backup {
          font-family: "JetBrains Mono", monospace;
          font-size: 1.125rem;
          letter-spacing: 0.1em;
          padding: 0.625rem;
        }
        .mfa-btn {
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
        .mfa-btn:hover:not(:disabled) { background: var(--brand-l); }
        .mfa-btn:disabled { opacity: 0.5; cursor: not-allowed; }
        .mfa-switch {
          background: none;
          border: none;
          color: var(--muted);
          font-size: 0.8125rem;
          cursor: pointer;
          padding: 0;
          margin-top: 1rem;
          text-decoration: underline;
          transition: color 0.15s;
        }
        .mfa-switch:hover { color: var(--text); }
      `}</style>
    </div>
  );
}
