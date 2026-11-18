import { useState } from "react";
import type { Employee } from "../lib/api";
import { useT } from "../lib/i18n";
import { EmployeesTab } from "./admin/EmployeesTab";
import { SchedulesTab } from "./admin/SchedulesTab";

export function Admin({ employee }: { employee: Employee }) {
  const { t } = useT();
  const [tab, setTab] = useState<"employees" | "schedules">("employees");

  return (
    <div className="shell">
      <header className="topbar">
        <span className="brand">{t.adminTitle}</span>
      </header>

      <main className="main admin-main with-nav">
        <div className="tabs" role="tablist" aria-label={t.adminTitle}>
          {(["employees", "schedules"] as const).map((k) => (
            <button
              key={k}
              role="tab"
              id={`tab-${k}`}
              aria-selected={tab === k}
              aria-controls={`panel-${k}`}
              className="tab"
              onClick={() => setTab(k)}
            >
              {k === "employees" ? t.tabEmployees : t.tabSchedules}
            </button>
          ))}
        </div>
        <div role="tabpanel" id={`panel-${tab}`} aria-labelledby={`tab-${tab}`}>
          {tab === "employees" ? (
            <EmployeesTab viewerRole={employee.role} viewerId={employee.id} />
          ) : (
            <SchedulesTab />
          )}
        </div>
      </main>
    </div>
  );
}
