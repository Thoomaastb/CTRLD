"use client";

import { useSystemInfo } from "@/lib/hooks/use-metrics";
import { useLogout, useRole } from "@/lib/hooks/use-auth";
import { PIMBadge } from "@/components/dashboard/pim-badge";
import { LogOut, User } from "lucide-react";

export function Topbar() {
  const { data: sysInfo } = useSystemInfo();
  const logout = useLogout();
  const role = useRole();
  const hostname = sysInfo?.system?.hostname ?? "—";

  return (
    <header className="topbar">
      <div className="topbar-left">
        <span className="topbar-hostname">{hostname}</span>
        {sysInfo?.system?.os && (
          <span className="topbar-os">{sysInfo.system.os}</span>
        )}
      </div>

      <div className="topbar-right">
        <PIMBadge />
        <div className="topbar-user">
          <User size={14} />
          <span className="topbar-role">{role ?? "—"}</span>
        </div>
        <button
          className="topbar-logout"
          onClick={() => logout.mutate()}
          title="Abmelden"
        >
          <LogOut size={14} />
        </button>
      </div>

      <style>{`
        .topbar {
          height: 44px;
          background: var(--surface);
          border-bottom: 0.5px solid var(--border);
          display: flex;
          align-items: center;
          justify-content: space-between;
          padding: 0 1.25rem;
          position: sticky;
          top: 0;
          z-index: 50;
        }

        .topbar-left {
          display: flex;
          align-items: center;
          gap: 0.75rem;
        }

        .topbar-hostname {
          font-size: 0.8125rem;
          font-weight: 600;
          color: var(--text);
          font-family: "JetBrains Mono", monospace;
        }

        .topbar-os {
          font-size: 0.75rem;
          color: var(--muted);
        }

        .topbar-right {
          display: flex;
          align-items: center;
          gap: 0.75rem;
        }

        .topbar-user {
          display: flex;
          align-items: center;
          gap: 0.375rem;
          color: var(--muted);
          font-size: 0.75rem;
        }

        .topbar-role {
          color: var(--muted);
          font-size: 0.75rem;
          text-transform: capitalize;
        }

        .topbar-logout {
          background: none;
          border: none;
          color: var(--muted);
          cursor: pointer;
          display: flex;
          align-items: center;
          padding: 0.25rem;
          border-radius: 4px;
          transition: color 0.15s, background 0.15s;
        }

        .topbar-logout:hover {
          color: var(--red);
          background: color-mix(in srgb, var(--red) 10%, transparent);
        }
      `}</style>
    </header>
  );
}
