"use client";

import { usePathname } from "next/navigation";
import Link from "next/link";
import {
  LayoutDashboard,
  Activity,
  ScrollText,
  Settings2,
  Users,
  LogOut,
  Shield,
} from "lucide-react";
import { cn } from "@/lib/utils/cn";
import { useLogout, useRole } from "@/lib/hooks/use-auth";

const navItems = [
  { href: "/dashboard", icon: LayoutDashboard, label: "Dashboard" },
  { href: "/monitoring", icon: Activity, label: "Monitoring" },
  { href: "/logs", icon: ScrollText, label: "Logs" },
  { href: "/services", icon: Settings2, label: "Services" },
];

const adminItems = [
  { href: "/users", icon: Users, label: "Benutzer" },
];

export function Sidebar() {
  const pathname = usePathname();
  const logout = useLogout();
  const role = useRole();
  const isAdmin = role === "admin";

  return (
    <aside className="sidebar">
      {/* Logo */}
      <Link href="/dashboard" className="sidebar-logo" title="CTRLD">
        <svg width="24" height="24" viewBox="0 0 32 32" fill="none">
          <rect width="32" height="32" rx="8" fill="var(--brand)" />
          <path
            d="M16 6L26 11V21L16 26L6 21V11L16 6Z"
            stroke="white"
            strokeWidth="2"
            strokeLinejoin="round"
          />
          <circle cx="16" cy="16" r="3" fill="white" />
        </svg>
      </Link>

      <div className="sidebar-divider" />

      {/* Navigation */}
      <nav className="sidebar-nav">
        {navItems.map(({ href, icon: Icon, label }) => (
          <SidebarItem
            key={href}
            href={href}
            icon={Icon}
            label={label}
            active={pathname.startsWith(href)}
          />
        ))}

        {isAdmin && (
          <>
            <div className="sidebar-divider sidebar-divider--mid" />
            {adminItems.map(({ href, icon: Icon, label }) => (
              <SidebarItem
                key={href}
                href={href}
                icon={Icon}
                label={label}
                active={pathname.startsWith(href)}
              />
            ))}
          </>
        )}
      </nav>

      {/* Bottom: Audit + Logout */}
      <div className="sidebar-bottom">
        {isAdmin && (
          <SidebarItem
            href="/audit"
            icon={Shield}
            label="Audit-Log"
            active={pathname.startsWith("/audit")}
          />
        )}
        <button
          className="sidebar-item sidebar-item--logout"
          onClick={() => logout.mutate()}
          title="Abmelden"
          aria-label="Abmelden"
        >
          <LogOut size={18} />
        </button>
      </div>

      <style>{`
        .sidebar {
          width: var(--sidebar-w);
          min-height: 100dvh;
          background: var(--surface);
          border-right: 0.5px solid var(--border);
          display: flex;
          flex-direction: column;
          align-items: center;
          padding: 0.75rem 0;
          position: fixed;
          left: 0;
          top: 0;
          z-index: 100;
        }

        .sidebar-logo {
          display: flex;
          align-items: center;
          justify-content: center;
          padding: 0.375rem;
          border-radius: 8px;
          transition: background 0.15s;
          margin-bottom: 0.5rem;
        }
        .sidebar-logo:hover { background: var(--border-sub); }

        .sidebar-divider {
          width: 28px;
          height: 0.5px;
          background: var(--border-sub);
          margin: 0.5rem 0;
        }
        .sidebar-divider--mid { margin: 0.375rem 0; }

        .sidebar-nav {
          display: flex;
          flex-direction: column;
          align-items: center;
          gap: 0.125rem;
          flex: 1;
          width: 100%;
          padding: 0 0.375rem;
        }

        .sidebar-bottom {
          display: flex;
          flex-direction: column;
          align-items: center;
          gap: 0.125rem;
          width: 100%;
          padding: 0 0.375rem;
        }

        .sidebar-item {
          width: 36px;
          height: 36px;
          display: flex;
          align-items: center;
          justify-content: center;
          border-radius: 8px;
          color: var(--muted);
          transition: color 0.15s, background 0.15s;
          position: relative;
          border: none;
          background: none;
          cursor: pointer;
          text-decoration: none;
        }

        .sidebar-item:hover {
          color: var(--text);
          background: var(--border-sub);
        }

        .sidebar-item--active {
          color: var(--brand-l);
          background: var(--brand-bg);
        }

        .sidebar-item--logout:hover {
          color: var(--red);
          background: color-mix(in srgb, var(--red) 10%, transparent);
        }

        /* Tooltip */
        .sidebar-item::after {
          content: attr(data-label);
          position: absolute;
          left: calc(100% + 8px);
          background: var(--surface);
          border: 0.5px solid var(--border);
          border-radius: 6px;
          padding: 0.25rem 0.625rem;
          font-size: 0.75rem;
          color: var(--text);
          white-space: nowrap;
          opacity: 0;
          pointer-events: none;
          transition: opacity 0.1s;
          z-index: 200;
        }
        .sidebar-item:hover::after { opacity: 1; }
      `}</style>
    </aside>
  );
}

interface SidebarItemProps {
  href: string;
  icon: React.ComponentType<{ size?: number }>;
  label: string;
  active: boolean;
}

function SidebarItem({ href, icon: Icon, label, active }: SidebarItemProps) {
  return (
    <Link
      href={href}
      className={cn("sidebar-item", active && "sidebar-item--active")}
      title={label}
      aria-label={label}
      data-label={label}
    >
      <Icon size={18} />
    </Link>
  );
}
