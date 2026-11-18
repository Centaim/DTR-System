import { useState } from "react";
import type { Employee } from "../lib/api";
import { useT } from "../lib/i18n";
import { EmployeesTab } from "./admin/EmployeesTab";
import { SchedulesTab } from "./admin/SchedulesTab";
import { AuditTab } from "./admin/AuditTab";
import { DashboardTab } from "./admin/DashboardTab";
import { ReportsTab } from "./admin/ReportsTab";
import { ActivityTab } from "./admin/ActivityTab";
import { KiosksTab } from "./admin/KiosksTab";
import { SettingsTab } from "./admin/SettingsTab";
import { TrackerTab } from "./admin/TrackerTab";
import { LocationsTab } from "./admin/LocationsTab";
import { ComplianceTab } from "./admin/ComplianceTab";
import { SystemTab } from "./admin/SystemTab";

export function Admin({ employee }: { employee: Employee }) {
  const { t } = useT();
  const manager = employee.role === "hr_admin" || employee.role === "super_admin";
  type Tab = "dashboard" | "employees" | "schedules" | "audit" | "reports" | "activity" | "kiosks" | "settings" | "tracker" | "locations" | "compliance" | "system";
  const tabs: Tab[] = manager
    ? ["dashboard", "tracker", "employees", "schedules", "kiosks", "locations", "audit", "reports", "compliance", "activity", "system", "settings"]
    : ["dashboard", "tracker", "audit", "reports", "compliance", "activity"];
  const [tab, setTab] = useState<Tab>("dashboard");
  const labels: Record<Tab, string> = {
    dashboard: t.tabDashboard,
    employees: t.tabEmployees,
    schedules: t.tabSchedules,
    audit: t.tabAudit,
    reports: t.tabReports,
    activity: t.tabActivity,
    kiosks: t.tabKiosks,
    settings: t.tabSettings,
    tracker: t.tabTracker,
    locations: t.tabLocations,
    compliance: t.tabCompliance,
    system: t.tabSystem,
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
          ) : tab === "activity" ? (
            <ActivityTab />
          ) : tab === "compliance" ? (
            <ComplianceTab canExportPeople={manager} />
          ) : tab === "system" ? (
            <SystemTab />
          ) : tab === "locations" ? (
            <LocationsTab />
          ) : tab === "tracker" ? (
            <TrackerTab canReopen={manager} />
          ) : tab === "settings" ? (
            <SettingsTab canEdit={employee.role === "super_admin"} />
          ) : tab === "kiosks" ? (
            <KiosksTab />
          ) : (
            <AuditTab canReview={manager} />
          )}
        </div>
      </main>
    </div>
  );
}
