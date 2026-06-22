import { Sidebar } from "@/components/layout/sidebar";
import { Topbar } from "@/components/layout/topbar";

export default function LogsLayout({ children }: { children: React.ReactNode }) {
  return (
    <div className="app-shell">
      <Sidebar />
      <div className="app-content">
        <Topbar />
        <main className="app-main app-main--logs">{children}</main>
      </div>
      <style>{`
        .app-shell { display: flex; min-height: 100dvh; background: var(--bg); }
        .app-content { margin-left: var(--sidebar-w); flex: 1; display: flex; flex-direction: column; min-width: 0; overflow: hidden; }
        .app-main--logs { flex: 1; padding: 1.25rem 1.5rem; overflow: hidden; display: flex; flex-direction: column; }
      `}</style>
    </div>
  );
}
