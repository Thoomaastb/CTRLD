import { Sidebar } from "@/components/layout/sidebar";

export default function DashboardLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  return (
    <div className="app-layout">
      <Sidebar />
      <main className="app-main">
        {children}
      </main>

      <style>{`
        .app-layout {
          display: flex;
          min-height: 100dvh;
          background: var(--bg);
        }

        .app-main {
          margin-left: var(--sidebar-w);
          flex: 1;
          padding: 1.5rem;
          min-width: 0;
        }
      `}</style>
    </div>
  );
}
