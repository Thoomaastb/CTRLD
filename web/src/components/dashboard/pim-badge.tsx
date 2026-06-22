"use client";

import { useEffect, useState } from "react";
import { Shield, ShieldAlert } from "lucide-react";
import { useQuery } from "@tanstack/react-query";
import { apiClient } from "@/lib/api/client";

interface PIMSession {
  active: boolean;
  session?: {
    id: string;
    reason: string;
    expires_at: string;
    remaining_seconds: number;
    is_break_glass: boolean;
  };
}

export function PIMBadge() {
  const { data } = useQuery<PIMSession>({
    queryKey: ["pim", "active"],
    queryFn: () => apiClient.get<PIMSession>("/pim/active"),
    refetchInterval: 10_000,
    retry: 0,
  });

  const [remaining, setRemaining] = useState(0);

  useEffect(() => {
    if (!data?.active || !data.session) return;
    setRemaining(data.session.remaining_seconds);

    const interval = setInterval(() => {
      setRemaining((prev) => Math.max(0, prev - 1));
    }, 1000);

    return () => clearInterval(interval);
  }, [data]);

  if (!data?.active || !data.session) return null;

  const { is_break_glass } = data.session;
  const mins = Math.floor(remaining / 60);
  const secs = remaining % 60;
  const timeStr = `${mins}:${secs.toString().padStart(2, "0")}`;

  return (
    <div className={`pim-badge ${is_break_glass ? "pim-badge--critical" : ""}`}>
      {is_break_glass ? (
        <ShieldAlert size={13} />
      ) : (
        <Shield size={13} />
      )}
      <span className="pim-badge__label">
        {is_break_glass ? "BREAK-GLASS" : "PIM"}
      </span>
      <span className="pim-badge__timer">{timeStr}</span>

      <style>{`
        .pim-badge {
          display: inline-flex;
          align-items: center;
          gap: 0.375rem;
          background: var(--brand-bg);
          color: var(--brand-t);
          border: 0.5px solid var(--brand);
          border-radius: 6px;
          padding: 0.25rem 0.625rem;
          font-size: 0.6875rem;
          font-weight: 600;
          letter-spacing: 0.04em;
          animation: pim-pulse 2s ease-in-out infinite;
        }

        .pim-badge--critical {
          background: color-mix(in srgb, var(--red) 12%, transparent);
          color: var(--red);
          border-color: var(--red);
        }

        .pim-badge__timer {
          font-family: "JetBrains Mono", monospace;
          font-size: 0.75rem;
          letter-spacing: -0.02em;
        }

        @keyframes pim-pulse {
          0%, 100% { opacity: 1; }
          50% { opacity: 0.75; }
        }
      `}</style>
    </div>
  );
}
