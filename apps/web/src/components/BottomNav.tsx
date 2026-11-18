import { Bell, CalendarDays, FilePenLine, House, Menu } from "lucide-react";
import { useT } from "../lib/i18n";

export type Screen =
  | "home" | "dtr" | "requests" | "alerts" | "more"
  | "approvals" | "admin" | "accomplishments" | "events" | "logbook";

const MAIN: Screen[] = ["home", "dtr", "requests", "alerts"];

export function BottomNav({
  screen,
  approvals,
  unread,
  onGo,
}: {
  screen: Screen;
  approvals: number;
  unread: number;
  onGo: (s: Screen) => void;
}) {
  const { t } = useT();
  const items: { id: Screen; label: string; icon: typeof House; badge?: number }[] = [
    { id: "home", label: t.navHome, icon: House },
    { id: "dtr", label: t.navDtr, icon: CalendarDays },
    { id: "requests", label: t.navRequests, icon: FilePenLine },
    { id: "alerts", label: t.navAlerts, icon: Bell, badge: unread },
    { id: "more", label: t.navMore, icon: Menu, badge: approvals },
  ];
  const current = MAIN.includes(screen) ? screen : "more";

  return (
    <nav className="bottom-nav no-print" aria-label={t.navLabel}>
      {items.map(({ id, label, icon: Icon, badge }) => (
        <button key={id} className="nav-item" aria-current={current === id ? "page" : undefined} onClick={() => onGo(id)}>
          <span className="nav-icon">
            <Icon aria-hidden="true" size={22} />
            {badge ? <span className="nav-badge">{badge}</span> : null}
          </span>
          <span>{label}</span>
        </button>
      ))}
    </nav>
  );
}
