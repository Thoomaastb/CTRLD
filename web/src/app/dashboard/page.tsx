import type { Metadata } from "next";

export const metadata: Metadata = {
  title: "Dashboard",
};

/**
 * Dashboard-Route — Platzhalter für v1.x Monitoring.
 * Wird in US-xxx mit echten Widgets befüllt.
 */
export default function DashboardPage() {
  return (
    <main className="dashboard-page">
      <h1>Dashboard</h1>
      <p>Monitoring-Widgets folgen in v1.x.</p>
    </main>
  );
}
