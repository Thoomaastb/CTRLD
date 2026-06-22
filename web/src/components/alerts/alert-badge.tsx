"use client";

import { Bell, BellRing } from "lucide-react";
import Link from "next/link";
import { useQuery } from "@tanstack/react-query";
import { apiClient } from "@/lib/api/client";
import { useEffect, useRef } from "react";
import { addToast } from "./toast";

interface AlertsResponse {
  alerts: Array<{
    id: string;
    type: string;
    severity: string;
    current_value: number;
    threshold: number;
    triggered_at: string;
  }>;
  count: number;
}

export function AlertBadge() {
  const { data } = useQuery<AlertsResponse>({
    queryKey: ["alerts", "active"],
    queryFn: () => apiClient.get<AlertsResponse>("/alerts"),
    refetchInterval: 15_000,
    retry: 0,
  });

  const prevCount = useRef(0);
  const count = data?.count ?? 0;
  const hasCritical = data?.alerts.some((a) => a.severity === "critical") ?? false;

  // Neuen Alert als Toast anzeigen
  useEffect(() => {
    if (data && data.count > prevCount.current && prevCount.current > 0) {
      const newest = data.alerts[0];
      if (newest) {
        addToast({
          type: newest.severity as "warning" | "critical",
          title: `Alert: ${newest.type.toUpperCase()} ${newest.severity === "critical" ? "⚠️ KRITISCH" : ""}`,
          message: `Aktueller Wert: ${newest.current_value.toFixed(1)}% (Schwellwert: ${newest.threshold}%)`,
        });
      }
    }
    prevCount.current = data?.count ?? 0;
  }, [data]);

  if (count === 0) {
    return (
      <Link href="/alerts" className="alert-badge-btn" title="Keine aktiven Alerts">
        <Bell size={15} />
        <style>{`.alert-badge-btn { display:flex; align-items:center; color:var(--muted); padding:0.25rem; border-radius:4px; transition:color 0.15s; } .alert-badge-btn:hover { color:var(--text); }`}</style>
      </Link>
    );
  }

  return (
    <Link
      href="/alerts"
      className={`alert-badge ${hasCritical ? "alert-badge--critical" : "alert-badge--warning"}`}
      title={`${count} aktive Alerts`}
    >
      <BellRing size={13} />
      <span className="alert-badge__count">{count}</span>

      <style>{`
        .alert-badge {
          display: inline-flex;
          align-items: center;
          gap: 0.3rem;
          padding: 0.25rem 0.5rem;
          border-radius: 6px;
          border: 0.5px solid;
          font-size: 0.6875rem;
          font-weight: 700;
          text-decoration: none;
          transition: all 0.15s;
          animation: alert-shake 0.5s ease;
        }

        .alert-badge--warning {
          color: var(--yellow);
          border-color: color-mix(in srgb, var(--yellow) 40%, transparent);
          background: color-mix(in srgb, var(--yellow) 10%, transparent);
        }

        .alert-badge--critical {
          color: var(--red);
          border-color: color-mix(in srgb, var(--red) 40%, transparent);
          background: color-mix(in srgb, var(--red) 10%, transparent);
          animation: alert-shake 0.5s ease, critical-pulse 2s ease-in-out infinite;
        }

        @keyframes alert-shake {
          0%,100% { transform: rotate(0deg); }
          20%      { transform: rotate(-8deg); }
          40%      { transform: rotate(8deg); }
          60%      { transform: rotate(-4deg); }
          80%      { transform: rotate(4deg); }
        }

        @keyframes critical-pulse {
          0%,100% { opacity: 1; }
          50%     { opacity: 0.7; }
        }
      `}</style>
    </Link>
  );
}
