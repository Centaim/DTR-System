import { CalendarDays, ClipboardCheck, FilePenLine, House, Users } from "lucide-react";
import { useT } from "../lib/i18n";

export type Screen = "home" | "dtr" | "requests" | "approvals" | "admin";

export function BottomNav({
  screen,
  role,
  approvals,
  onGo,
}: {
  screen: Screen;
  role: string;
  approvals: number;
  onGo: (s: Screen) => void;
}) {
  const { t } = useT();
  const approver = role === "supervisor" || role === "hr_admin" || role === "super_admin";
  const admin = role === "hr_admin" || role === "super_admin";

  const items: { id: Screen; label: string; icon: typeof House; badge?: number }[] = [
    { id: "home", label: t.navHome, icon: House },
    { id: "dtr", label: t.navDtr, icon: CalendarDays },
    { id: "requests", label: t.navRequests, icon: FilePenLine },
    ...(approver ? [{ id: "approvals" as const, label: t.navApprovals, icon: ClipboardCheck, badge: approvals }] : []),
    ...(admin ? [{ id: "admin" as const, label: t.navAdmin, icon: Users }] : []),
  ];

  return (
    <nav className="bottom-nav no-print" aria-label={t.navLabel}>
      {items.map(({ id, label, icon: Icon, badge }) => (
        <button
          key={id}
          className="nav-item"
          aria-current={screen === id ? "page" : undefined}
          onClick={() => onGo(id)}
        >
          <span className="nav-icon">
            <Icon aria-hidden="true" size={22} />
            {badge ? <span className="nav-badge" aria-label={`${badge}`}>{badge}</span> : null}
          </span>
          <span>{label}</span>
        </button>
      ))}
    </nav>
  );
}
