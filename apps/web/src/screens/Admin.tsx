import { useState } from "react";
import type { Employee } from "../lib/api";
import { useT } from "../lib/i18n";
import { EmployeesTab } from "./admin/EmployeesTab";
import { SchedulesTab } from "./admin/SchedulesTab";
import { AuditTab } from "./admin/AuditTab";

export function Admin({ employee }: { employee: Employee }) {
  const { t } = useT();
  const manager = employee.role === "hr_admin" || employee.role === "super_admin";
  const tabs = (manager ? ["employees", "schedules", "audit"] : ["audit"]) as ("employees" | "schedules" | "audit")[];
  const [tab, setTab] = useState<"employees" | "schedules" | "audit">(tabs[0]!);
  const label = (k: string) => (k === "employees" ? t.tabEmployees : k === "schedules" ? t.tabSchedules : t.tabAudit);

  return (
    <div className="shell">
      <header className="topbar">
        <span className="brand">{t.adminTitle}</span>
      </header>

      <main className="main admin-main with-nav">
        <div className="tabs" role="tablist" aria-label={t.adminTitle}>
          {tabs.map((k) => (
            <button
              key={k}
              role="tab"
              id={`tab-${k}`}
              aria-selected={tab === k}
              aria-controls={`panel-${k}`}
              className="tab"
              onClick={() => setTab(k)}
            >
              {label(k)}
            </button>
          ))}
        </div>
        <div role="tabpanel" id={`panel-${tab}`} aria-labelledby={`tab-${tab}`}>
          {tab === "employees" ? (
            <EmployeesTab viewerRole={employee.role} viewerId={employee.id} />
          ) : tab === "schedules" ? (
            <SchedulesTab />
          ) : (
            <AuditTab canReview={manager} />
          )}
        </div>
      </main>
    </div>
  );
}
