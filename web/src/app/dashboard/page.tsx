import type { Metadata } from "next";

export const metadata: Metadata = { title: "Dashboard" };

export default function DashboardPage() {
  return (
    <div className="dashboard">
      <div className="dashboard-header">
        <h1 className="dashboard-title">Dashboard</h1>
        <p className="dashboard-subtitle">Willkommen bei CTRLD</p>
      </div>

      <div className="dashboard-grid">
        {/* Platzhalter-Widgets — werden in v0.x Monitoring befüllt */}
        {["CPU", "RAM", "Disk", "Netzwerk", "Uptime", "Services"].map((label) => (
          <div key={label} className="widget">
            <div className="widget-label">{label}</div>
            <div className="widget-value widget-value--placeholder">—</div>
          </div>
        ))}
      </div>

      <style>{`
        .dashboard { }

        .dashboard-header {
          margin-bottom: 1.5rem;
        }

        .dashboard-title {
          font-size: 1.25rem;
          font-weight: 600;
          color: var(--text);
          margin: 0 0 0.25rem;
        }

        .dashboard-subtitle {
          font-size: 0.8125rem;
          color: var(--muted);
          margin: 0;
        }

        .dashboard-grid {
          display: grid;
          grid-template-columns: repeat(auto-fill, minmax(200px, 1fr));
          gap: 1rem;
        }

        .widget {
          background: var(--surface);
          border: 0.5px solid var(--border);
          border-radius: var(--radius);
          padding: 1.25rem;
        }

        .widget-label {
          font-size: 0.75rem;
          font-weight: 500;
          color: var(--muted);
          text-transform: uppercase;
          letter-spacing: 0.05em;
          margin-bottom: 0.625rem;
        }

        .widget-value {
          font-size: 1.5rem;
          font-weight: 600;
          color: var(--text);
          font-family: "JetBrains Mono", monospace;
        }

        .widget-value--placeholder {
          color: var(--dim);
        }
      `}</style>
    </div>
  );
}
