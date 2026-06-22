"use client";

import { Sparkline } from "./sparkline";
import { cn } from "@/lib/utils/cn";

interface MetricCardProps {
  label: string;
  value: string;
  subValue?: string;
  percent?: number;
  history?: number[];
  color?: string;
  status?: "ok" | "warning" | "critical";
  icon?: React.ReactNode;
  className?: string;
}

const statusColor = {
  ok:       "var(--green)",
  warning:  "var(--yellow)",
  critical: "var(--red)",
};

/**
 * Metriken-Widget mit optionalem Sparkline-Chart und Status-Farbe.
 */
export function MetricCard({
  label,
  value,
  subValue,
  percent,
  history = [],
  color,
  status = "ok",
  icon,
  className,
}: MetricCardProps) {
  const activeColor = color ?? statusColor[status];

  return (
    <div className={cn("metric-card", className)}>
      {/* Header */}
      <div className="metric-card__header">
        <div className="metric-card__label">
          {icon && <span className="metric-card__icon">{icon}</span>}
          {label}
        </div>
        {percent !== undefined && (
          <span
            className="metric-card__percent"
            style={{ color: activeColor }}
          >
            {percent.toFixed(1)}%
          </span>
        )}
      </div>

      {/* Wert */}
      <div className="metric-card__value">{value}</div>
      {subValue && <div className="metric-card__sub">{subValue}</div>}

      {/* Fortschrittsbalken */}
      {percent !== undefined && (
        <div className="metric-card__bar-track">
          <div
            className="metric-card__bar-fill"
            style={{
              width: `${Math.min(percent, 100)}%`,
              background: activeColor,
            }}
          />
        </div>
      )}

      {/* Sparkline */}
      {history.length > 1 && (
        <div className="metric-card__sparkline">
          <Sparkline
            data={history}
            width={180}
            height={28}
            color={activeColor}
            max={100}
          />
        </div>
      )}

      <style>{`
        .metric-card {
          background: var(--surface);
          border: 0.5px solid var(--border);
          border-radius: var(--radius);
          padding: 1rem 1.125rem;
          display: flex;
          flex-direction: column;
          gap: 0.375rem;
          transition: border-color 0.2s;
        }

        .metric-card:hover {
          border-color: var(--brand);
        }

        .metric-card__header {
          display: flex;
          align-items: center;
          justify-content: space-between;
        }

        .metric-card__label {
          font-size: 0.6875rem;
          font-weight: 600;
          color: var(--muted);
          text-transform: uppercase;
          letter-spacing: 0.07em;
          display: flex;
          align-items: center;
          gap: 0.375rem;
        }

        .metric-card__icon {
          opacity: 0.7;
          display: flex;
          align-items: center;
        }

        .metric-card__percent {
          font-size: 0.75rem;
          font-weight: 700;
          font-family: "JetBrains Mono", monospace;
          letter-spacing: -0.02em;
        }

        .metric-card__value {
          font-size: 1.375rem;
          font-weight: 700;
          color: var(--text);
          font-family: "JetBrains Mono", monospace;
          letter-spacing: -0.03em;
          line-height: 1.2;
        }

        .metric-card__sub {
          font-size: 0.75rem;
          color: var(--muted);
          font-family: "JetBrains Mono", monospace;
        }

        .metric-card__bar-track {
          height: 3px;
          background: var(--border-sub);
          border-radius: 2px;
          overflow: hidden;
          margin-top: 0.25rem;
        }

        .metric-card__bar-fill {
          height: 100%;
          border-radius: 2px;
          transition: width 0.5s ease;
        }

        .metric-card__sparkline {
          margin-top: 0.25rem;
          opacity: 0.8;
        }
      `}</style>
    </div>
  );
}
