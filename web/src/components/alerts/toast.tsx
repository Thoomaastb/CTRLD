"use client";

import { useEffect, useState, useCallback } from "react";
import { X, AlertTriangle, AlertOctagon, CheckCircle } from "lucide-react";

export interface ToastMessage {
  id: string;
  type: "warning" | "critical" | "resolved";
  title: string;
  message: string;
  timestamp: Date;
}

interface ToastProps {
  toast: ToastMessage;
  onDismiss: (id: string) => void;
}

const toastConfig = {
  warning:  { icon: AlertTriangle,  color: "var(--yellow)", bg: "color-mix(in srgb, var(--yellow) 10%, transparent)", border: "color-mix(in srgb, var(--yellow) 30%, transparent)" },
  critical: { icon: AlertOctagon,   color: "var(--red)",    bg: "color-mix(in srgb, var(--red) 10%, transparent)",    border: "color-mix(in srgb, var(--red) 30%, transparent)"    },
  resolved: { icon: CheckCircle,    color: "var(--green)",  bg: "color-mix(in srgb, var(--green) 10%, transparent)",  border: "color-mix(in srgb, var(--green) 30%, transparent)"  },
};

function Toast({ toast, onDismiss }: ToastProps) {
  const cfg = toastConfig[toast.type];
  const Icon = cfg.icon;

  useEffect(() => {
    const timer = setTimeout(() => onDismiss(toast.id),
      toast.type === "critical" ? 10_000 : 6_000);
    return () => clearTimeout(timer);
  }, [toast.id, toast.type, onDismiss]);

  return (
    <div
      className="toast"
      style={{ background: cfg.bg, borderColor: cfg.border }}
      role="alert"
      aria-live="assertive"
    >
      <div className="toast-icon" style={{ color: cfg.color }}>
        <Icon size={16} />
      </div>
      <div className="toast-content">
        <div className="toast-title" style={{ color: cfg.color }}>{toast.title}</div>
        <div className="toast-message">{toast.message}</div>
      </div>
      <button className="toast-close" onClick={() => onDismiss(toast.id)}>
        <X size={12} />
      </button>

      <style>{`
        .toast {
          display: flex;
          align-items: flex-start;
          gap: 0.625rem;
          border: 0.5px solid;
          border-radius: var(--radius);
          padding: 0.75rem 0.875rem;
          min-width: 300px;
          max-width: 420px;
          animation: toast-in 0.2s ease forwards;
          backdrop-filter: blur(8px);
        }

        @keyframes toast-in {
          from { opacity: 0; transform: translateX(100%); }
          to   { opacity: 1; transform: translateX(0); }
        }

        .toast-icon { flex-shrink: 0; margin-top: 1px; }

        .toast-content { flex: 1; min-width: 0; }

        .toast-title {
          font-size: 0.8125rem;
          font-weight: 600;
          margin-bottom: 0.2rem;
        }

        .toast-message {
          font-size: 0.75rem;
          color: var(--muted);
          line-height: 1.4;
        }

        .toast-close {
          background: none;
          border: none;
          color: var(--muted);
          cursor: pointer;
          padding: 0.125rem;
          border-radius: 3px;
          flex-shrink: 0;
          transition: color 0.15s;
        }

        .toast-close:hover { color: var(--text); }
      `}</style>
    </div>
  );
}

// ── Toast Container ───────────────────────────────────────────────────────────

let globalAddToast: ((toast: Omit<ToastMessage, "id" | "timestamp">) => void) | null = null;

export function addToast(toast: Omit<ToastMessage, "id" | "timestamp">) {
  globalAddToast?.(toast);
}

export function ToastContainer() {
  const [toasts, setToasts] = useState<ToastMessage[]>([]);

  const add = useCallback((toast: Omit<ToastMessage, "id" | "timestamp">) => {
    const id = Math.random().toString(36).slice(2);
    setToasts((prev) => [
      ...prev.slice(-4), // Max 5 gleichzeitig
      { ...toast, id, timestamp: new Date() },
    ]);
  }, []);

  useEffect(() => {
    globalAddToast = add;
    return () => { globalAddToast = null; };
  }, [add]);

  const dismiss = useCallback((id: string) => {
    setToasts((prev) => prev.filter((t) => t.id !== id));
  }, []);

  if (toasts.length === 0) return null;

  return (
    <div className="toast-container">
      {toasts.map((t) => (
        <Toast key={t.id} toast={t} onDismiss={dismiss} />
      ))}

      <style>{`
        .toast-container {
          position: fixed;
          bottom: 1.25rem;
          right: 1.25rem;
          display: flex;
          flex-direction: column;
          gap: 0.5rem;
          z-index: 9999;
        }
      `}</style>
    </div>
  );
}
