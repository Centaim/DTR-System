import { useState } from "react";
import type { Employee } from "../lib/api";
import { useT } from "../lib/i18n";
import { EmployeesTab } from "./admin/EmployeesTab";
import { SchedulesTab } from "./admin/SchedulesTab";
import { AuditTab } from "./admin/AuditTab";
import { DashboardTab } from "./admin/DashboardTab";
import { ReportsTab } from "./admin/ReportsTab";

export function Admin({ employee }: { employee: Employee }) {
  const { t } = useT();
  const manager = employee.role === "hr_admin" || employee.role === "super_admin";
  type Tab = "dashboard" | "employees" | "schedules" | "audit" | "reports";
  const tabs: Tab[] = manager
    ? ["dashboard", "employees", "schedules", "audit", "reports"]
    : ["dashboard", "audit", "reports"];
  const [tab, setTab] = useState<Tab>("dashboard");
  const labels: Record<Tab, string> = {
    dashboard: t.tabDashboard,
    employees: t.tabEmployees,
    schedules: t.tabSchedules,
    audit: t.tabAudit,
    reports: t.tabReports,
  };
  const label = (k: Tab) => labels[k];

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
          {tab === "dashboard" ? (
            <DashboardTab />
          ) : tab === "employees" ? (
            <EmployeesTab viewerRole={employee.role} viewerId={employee.id} />
          ) : tab === "schedules" ? (
            <SchedulesTab />
          ) : tab === "reports" ? (
            <ReportsTab />
          ) : (
            <AuditTab canReview={manager} />
          )}
        </div>
      </main>
    </div>
  );
}
