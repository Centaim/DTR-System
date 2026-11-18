import { useState } from "react";
import { CalendarCheck, ChevronRight, ClipboardCheck, ClipboardList, Download, NotebookPen, Users } from "lucide-react";
import { api, type Employee } from "../lib/api";
import { saveBlob } from "../lib/download";
import { useT } from "../lib/i18n";
import type { Screen } from "../components/BottomNav";

export function More({ employee, approvals, onGo }: { employee: Employee; approvals: number; onGo: (s: Screen) => void }) {
  const { t } = useT();
  const r = employee.role;
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const exportData = async () => {
    setBusy(true);
    setError("");
    try {
      saveBlob(await api.compliance.myData(), `my-dtr-data-${employee.employee_no}.zip`);
    } catch {
      setError(t.myDataFailed);
    } finally {
      setBusy(false);
    }
  };
  const items: { id: Screen; label: string; icon: typeof Users; show: boolean; badge?: number }[] = [
    { id: "approvals", label: t.moreApprovals, icon: ClipboardCheck, show: ["supervisor", "hr_admin", "super_admin"].includes(r), badge: approvals },
    { id: "accomplishments", label: t.moreAccomp, icon: ClipboardList, show: true },
    { id: "events", label: t.moreEvents, icon: CalendarCheck, show: true },
    { id: "logbook", label: t.moreLogbook, icon: NotebookPen, show: ["timekeeper", "hr_admin", "super_admin"].includes(r) },
    { id: "admin", label: t.moreAdmin, icon: Users, show: ["hr_admin", "super_admin", "auditor"].includes(r) },
  ];
  return (
    <div className="shell">
      <header className="topbar"><span className="brand">{t.moreTitle}</span></header>
      <main className="main narrow with-nav">
        <ul className="panel menu">
          {items.filter((i) => i.show).map(({ id, label, icon: Icon, badge }) => (
            <li key={id}>
              <button className="menu-item" onClick={() => onGo(id)}>
                <Icon aria-hidden="true" size={22} />
                <span>{label}</span>
                {badge ? <span className="tag status-pending">{badge}</span> : null}
                <ChevronRight aria-hidden="true" size={20} />
              </button>
            </li>
          ))}
          <li>
            <button className="menu-item" onClick={exportData} disabled={busy}>
              <Download aria-hidden="true" size={22} />
              <span>{busy ? t.myDataPreparing : t.myDataTitle}</span>
            </button>
          </li>
        </ul>
        <div aria-live="polite">{error && <p className="msg bad">{error}</p>}</div>
      </main>
    </div>
  );
}
