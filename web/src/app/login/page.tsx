"use client";

import { useState } from "react";
import { useLogin } from "@/lib/hooks/use-auth";
import { cn } from "@/lib/utils/cn";

export default function LoginPage() {
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const login = useLogin();

  function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    login.mutate({ email, password });
  }

  return (
    <div className="login-root">
      <div className="login-card">
        {/* Logo + Title */}
        <div className="login-header">
          <div className="login-logo">
            <svg width="32" height="32" viewBox="0 0 32 32" fill="none">
              <rect width="32" height="32" rx="8" fill="var(--brand)" />
              <path
                d="M16 6L26 11V21L16 26L6 21V11L16 6Z"
                stroke="white"
                strokeWidth="2"
                strokeLinejoin="round"
              />
              <circle cx="16" cy="16" r="3" fill="white" />
            </svg>
          </div>
          <h1 className="login-title">CTRLD</h1>
          <p className="login-subtitle">Server Control Panel</p>
        </div>

        {/* Fehler */}
        {login.isError && (
          <div className="login-error" role="alert">
            Ungültige Anmeldedaten. Bitte erneut versuchen.
          </div>
        )}

        {/* Formular */}
        <form onSubmit={handleSubmit} className="login-form">
          <div className="field">
            <label htmlFor="email" className="field-label">
              E-Mail
            </label>
            <input
              id="email"
              type="email"
              autoComplete="email"
              required
              value={email}
              onChange={(e) => setEmail(e.target.value)}
              className={cn("field-input", login.isError && "field-input--error")}
              placeholder="admin@example.com"
              disabled={login.isPending}
            />
          </div>

          <div className="field">
            <label htmlFor="password" className="field-label">
              Passwort
            </label>
            <input
              id="password"
              type="password"
              autoComplete="current-password"
              required
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              className={cn("field-input", login.isError && "field-input--error")}
              placeholder="••••••••••••"
              disabled={login.isPending}
            />
          </div>

          <button
            type="submit"
            disabled={login.isPending || !email || !password}
            className="login-btn"
          >
            {login.isPending ? (
              <span className="login-btn__loading">
                <span className="spinner" aria-hidden="true" />
                Anmelden…
              </span>
            ) : (
              "Anmelden"
            )}
          </button>
        </form>
      </div>

      <style>{`
        .login-root {
          min-height: 100dvh;
          display: flex;
          align-items: center;
          justify-content: center;
          background: var(--bg);
          padding: 1rem;
        }

        .login-card {
          width: 100%;
          max-width: 380px;
          background: var(--surface);
          border: 0.5px solid var(--border);
          border-radius: var(--radius);
          padding: 2rem;
        }

        .login-header {
          text-align: center;
          margin-bottom: 2rem;
        }

        .login-logo {
          display: inline-flex;
          margin-bottom: 0.75rem;
        }

        .login-title {
          font-size: 1.5rem;
          font-weight: 700;
          letter-spacing: 0.05em;
          color: var(--text);
          margin: 0 0 0.25rem;
        }

        .login-subtitle {
          font-size: 0.8125rem;
          color: var(--muted);
          margin: 0;
        }

        .login-error {
          background: color-mix(in srgb, var(--red) 12%, transparent);
          border: 0.5px solid color-mix(in srgb, var(--red) 40%, transparent);
          color: var(--red);
          border-radius: 6px;
          padding: 0.625rem 0.875rem;
          font-size: 0.8125rem;
          margin-bottom: 1.25rem;
        }

        .login-form {
          display: flex;
          flex-direction: column;
          gap: 1rem;
        }

        .field {
          display: flex;
          flex-direction: column;
          gap: 0.375rem;
        }

        .field-label {
          font-size: 0.8125rem;
          color: var(--muted);
          font-weight: 500;
        }

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

        .field-input:focus {
          border-color: var(--brand-l);
        }

        .field-input--error {
          border-color: var(--red);
        }

        .field-input:disabled {
          opacity: 0.5;
          cursor: not-allowed;
        }

        .login-btn {
          background: var(--brand);
          color: white;
          border: none;
          border-radius: 6px;
          font-size: 0.875rem;
          font-weight: 600;
          padding: 0.625rem 1rem;
          cursor: pointer;
          transition: background 0.15s, opacity 0.15s;
          margin-top: 0.5rem;
          width: 100%;
        }

        .login-btn:hover:not(:disabled) {
          background: var(--brand-l);
        }

        .login-btn:disabled {
          opacity: 0.5;
          cursor: not-allowed;
        }

        .login-btn__loading {
          display: flex;
          align-items: center;
          justify-content: center;
          gap: 0.5rem;
        }

        .spinner {
          width: 14px;
          height: 14px;
          border: 2px solid rgba(255,255,255,0.3);
          border-top-color: white;
          border-radius: 50%;
          animation: spin 0.6s linear infinite;
          display: inline-block;
        }

        @keyframes spin {
          to { transform: rotate(360deg); }
        }
      `}</style>
    </div>
  );
}
