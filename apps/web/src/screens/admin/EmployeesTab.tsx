import { useCallback, useEffect, useState, type FormEvent } from "react";
import { Plus, ChevronLeft, FileUp, IdCard } from "lucide-react";
import {
  ApiError,
  NetworkError,
  api,
  type AdminEmployee,
  type EmployeeInput,
  type Schedule,
} from "../../lib/api";
import { useT, type Strings } from "../../lib/i18n";
import { TempPin } from "./TempPin";
import { ImportScreen } from "./ImportScreen";
import { IdCards } from "./IdCards";

const ROLES = ["employee", "supervisor", "timekeeper", "auditor", "hr_admin", "super_admin"] as const;
const EMPLOYMENT = ["plantilla", "casual", "contract_of_service", "job_order"] as const;
const privileged = (r: string) => r === "hr_admin" || r === "super_admin";

const roleLabel = (t: Strings, r: string) => (t[`role_${r}` as keyof Strings] as string) ?? r;
const empLabel = (t: Strings, e: string) => (t[`emp_${e}` as keyof Strings] as string) ?? e;

const today = () =>
  new Intl.DateTimeFormat("en-CA", { timeZone: "Asia/Manila" }).format(new Date());

type Mode = { kind: "list" } | { kind: "create" } | { kind: "import" } | { kind: "cards" } | { kind: "edit"; emp: AdminEmployee };

function message(e: unknown, t: Strings): string {
  if (e instanceof NetworkError) return t.errNetwork;
  if (e instanceof ApiError && e.message) return e.message;
  return t.errGeneric;
}

export function EmployeesTab({ viewerRole, viewerId }: { viewerRole: string; viewerId: number }) {
  const { t } = useT();
  const [q, setQ] = useState("");
  const [list, setList] = useState<AdminEmployee[] | null>(null);
  const [schedules, setSchedules] = useState<Schedule[]>([]);
  const [failed, setFailed] = useState(false);
  const [mode, setMode] = useState<Mode>({ kind: "list" });
  const [pin, setPin] = useState<{ name: string; pin: string } | null>(null);

  const load = useCallback(() => {
    setFailed(false);
    api.admin.employees(q).then(setList).catch(() => setFailed(true));
  }, [q]);

  useEffect(() => {
    const id = window.setTimeout(load, 250);
    return () => window.clearTimeout(id);
  }, [load]);

  useEffect(() => {
    api.admin.schedules().then(setSchedules).catch(() => undefined);
  }, []);

  if (pin) {
    return (
      <TempPin
        name={pin.name}
        pin={pin.pin}
        onDone={() => {
          setPin(null);
          setMode({ kind: "list" });
          load();
        }}
      />
    );
  }

  const backToList = () => {
    setMode({ kind: "list" });
    load();
  };
  if (mode.kind === "import") return <ImportScreen onBack={backToList} />;
  if (mode.kind === "cards") return <IdCards onBack={backToList} />;

  if (mode.kind !== "list") {
    return (
      <EmployeeForm
        key={mode.kind === "edit" ? mode.emp.id : "new"}
        emp={mode.kind === "edit" ? mode.emp : null}
        all={list ?? []}
        schedules={schedules}
        viewerRole={viewerRole}
        viewerId={viewerId}
        onBack={() => {
          setMode({ kind: "list" });
          load();
        }}
        onTempPin={(name, p) => setPin({ name, pin: p })}
      />
    );
  }

  return (
    <section className="panel" aria-labelledby="emp-h">
      <div className="panel-bar">
        <h2 id="emp-h" className="panel-h">
          {t.tabEmployees}
        </h2>
        <div className="actions tight">
          <button className="btn primary" onClick={() => setMode({ kind: "create" })}>
            <Plus aria-hidden="true" size={20} /> {t.addEmployee}
          </button>
          <button className="btn quiet" onClick={() => setMode({ kind: "import" })}>
            <FileUp aria-hidden="true" size={20} /> {t.importEmployees}
          </button>
          <button className="btn quiet" onClick={() => setMode({ kind: "cards" })}>
            <IdCard aria-hidden="true" size={20} /> {t.idCards}
          </button>
        </div>
      </div>
      <label className="field">
        <span className="sr-only">{t.search}</span>
        <input type="search" placeholder={t.search} value={q} onChange={(e) => setQ(e.target.value)} />
      </label>

      {failed && (
        <p className="msg bad" role="alert">
          {t.loadFailed} <button className="link-btn" onClick={load}>{t.retry}</button>
        </p>
      )}
      {list && list.length === 0 && <p className="muted">{t.noEmployees}</p>}
      <ul className="rows">
        {list?.map((e) => (
          <li key={e.id}>
            <button className="row row-btn" onClick={() => setMode({ kind: "edit", emp: e })}>
              <span className="row-main">
                <span className="rail-slot">
                  {e.full_name} {!e.active && <span className="tag bad">{t.inactive}</span>}
                </span>
                <span className="rail-state">
                  {e.employee_no} · {e.position || "—"} · {e.office || "—"}
                </span>
              </span>
              <span className="row-side">
                <span className="tag">{roleLabel(t, e.role)}</span>
                <span className="rail-state">{e.schedule}</span>
              </span>
            </button>
          </li>
        ))}
      </ul>
    </section>
  );
}

function EmployeeForm({
  emp,
  all,
  schedules,
  viewerRole,
  viewerId,
  onBack,
  onTempPin,
}: {
  emp: AdminEmployee | null;
  all: AdminEmployee[];
  schedules: Schedule[];
  viewerRole: string;
  viewerId: number;
  onBack: () => void;
  onTempPin: (name: string, pin: string) => void;
}) {
  const { t } = useT();
  const [f, setF] = useState({
    employee_no: emp?.employee_no ?? "",
    full_name: emp?.full_name ?? "",
    position: emp?.position ?? "",
    office: emp?.office ?? "",
    role: emp?.role ?? "employee",
    employment: emp?.employment ?? "plantilla",
    supervisor_id: emp?.supervisor_id ?? 0,
  });
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const [saved, setSaved] = useState("");
  const [confirmReset, setConfirmReset] = useState(false);
  const [sched, setSched] = useState({ id: 0, from: today() });

  const locked = emp !== null && privileged(emp.role) && viewerRole !== "super_admin";
  const self = emp?.id === viewerId;
  const roles = viewerRole === "super_admin" ? ROLES : ROLES.filter((r) => !privileged(r));
  const supervisors = all.filter((e) => e.active && e.id !== emp?.id);

  const submit = async (ev: FormEvent) => {
    ev.preventDefault();
    setBusy(true);
    setError("");
    setSaved("");
    try {
      const body: EmployeeInput = {
        full_name: f.full_name,
        position: f.position,
        office: f.office,
        employment: f.employment,
        supervisor_id: f.supervisor_id,
      };
      if (!self) body.role = f.role;
      if (emp) {
        await api.admin.updateEmployee(emp.id, body);
        onBack();
      } else {
        const res = await api.admin.createEmployee({ ...body, employee_no: f.employee_no, role: f.role });
        onTempPin(res.employee.full_name, res.temp_pin);
      }
    } catch (e) {
      setError(message(e, t));
    } finally {
      setBusy(false);
    }
  };

  const run = async (fn: () => Promise<void>) => {
    setBusy(true);
    setError("");
    setSaved("");
    try {
      await fn();
    } catch (e) {
      setError(message(e, t));
    } finally {
      setBusy(false);
      setConfirmReset(false);
    }
  };

  const text = (key: "employee_no" | "full_name" | "position" | "office", label: string, opts?: { readOnly?: boolean; required?: boolean }) => (
    <label className="field">
      <span>{label}</span>
      <input
        value={f[key]}
        onChange={(e) => setF({ ...f, [key]: e.target.value })}
        readOnly={opts?.readOnly || locked}
        required={opts?.required}
        maxLength={120}
        autoCapitalize={key === "employee_no" ? "characters" : undefined}
      />
    </label>
  );

  return (
    <div className="admin-cols">
      <form className="panel form" onSubmit={submit} noValidate>
        <button type="button" className="link-btn back" onClick={onBack}>
          <ChevronLeft aria-hidden="true" size={18} /> {t.backToList}
        </button>
        <h2 className="title">{emp ? t.editEmployee : t.createEmployee}</h2>
        {locked && <p className="msg warn">{t.privilegedLocked}</p>}

        {text("employee_no", t.employeeNo, { readOnly: emp !== null, required: true })}
        {text("full_name", t.fullName, { required: true })}
        {text("position", t.position)}
        {text("office", t.office)}

        <label className="field">
          <span>{t.role}</span>
          <select
            value={f.role}
            disabled={locked || self}
            onChange={(e) => setF({ ...f, role: e.target.value })}
          >
            {(roles.includes(f.role as never) ? roles : [...roles, f.role as (typeof ROLES)[number]]).map((r) => (
              <option key={r} value={r}>
                {roleLabel(t, r)}
              </option>
            ))}
          </select>
        </label>
        <label className="field">
          <span>{t.employment}</span>
          <select value={f.employment} disabled={locked} onChange={(e) => setF({ ...f, employment: e.target.value })}>
            {EMPLOYMENT.map((x) => (
              <option key={x} value={x}>
                {empLabel(t, x)}
              </option>
            ))}
          </select>
        </label>
        <label className="field">
          <span>{t.supervisor}</span>
          <select
            value={f.supervisor_id}
            disabled={locked}
            onChange={(e) => setF({ ...f, supervisor_id: Number(e.target.value) })}
          >
            <option value={0}>{t.noSupervisor}</option>
            {supervisors.map((s) => (
              <option key={s.id} value={s.id}>
                {s.full_name}
              </option>
            ))}
          </select>
        </label>

        <div aria-live="polite">{error && <p className="msg bad">{error}</p>}</div>
        <button className="btn primary" type="submit" disabled={busy || locked || !f.full_name.trim() || !f.employee_no.trim()}>
          {busy ? t.saving : emp ? t.save : t.createEmployee}
        </button>
      </form>

      {emp && !locked && (
        <div className="stack">
          <section className="panel form" aria-labelledby="sc-h">
            <h2 id="sc-h" className="panel-h">
              {t.scheduleSection}
            </h2>
            <p className="muted">{t.currentSchedule(emp.schedule ?? "—")}</p>
            <label className="field">
              <span>{t.chooseSchedule}</span>
              <select value={sched.id} onChange={(e) => setSched({ ...sched, id: Number(e.target.value) })}>
                <option value={0} disabled>
                  {t.chooseSchedule}
                </option>
                {schedules.map((s) => (
                  <option key={s.id} value={s.id}>
                    {s.name}
                  </option>
                ))}
              </select>
            </label>
            <label className="field">
              <span>{t.scheduleFrom}</span>
              <input type="date" value={sched.from} onChange={(e) => setSched({ ...sched, from: e.target.value })} />
            </label>
            <button
              className="btn quiet"
              disabled={busy || sched.id === 0 || !sched.from}
              onClick={() =>
                run(async () => {
                  await api.admin.assignSchedule(emp.id, sched.id, sched.from);
                  setSaved(t.scheduleAssigned);
                })
              }
            >
              {t.assignSchedule}
            </button>
            {saved && (
              <p className="msg ok" role="status">
                {saved}
              </p>
            )}
          </section>

          <section className="panel form" aria-labelledby="pin-h">
            <h2 id="pin-h" className="panel-h">
              {t.pin}
            </h2>
            <p className="muted small">{t.resetPinNote}</p>
            {confirmReset ? (
              <button
                className="btn primary"
                disabled={busy}
                onClick={() =>
                  run(async () => {
                    const r = await api.admin.resetPin(emp.id);
                    onTempPin(emp.full_name, r.temp_pin);
                  })
                }
              >
                {t.confirmResetPin}
              </button>
            ) : (
              <button className="btn quiet" onClick={() => setConfirmReset(true)}>
                {t.resetPin}
              </button>
            )}
          </section>

          {!self && (
            <section className="panel form danger-zone">
              <p className="muted small">{t.deactivateNote}</p>
              <button
                className="btn quiet"
                disabled={busy}
                onClick={() =>
                  run(async () => {
                    await api.admin.updateEmployee(emp.id, { active: !emp.active });
                    onBack();
                  })
                }
              >
                {emp.active ? t.deactivate : t.reactivate}
              </button>
            </section>
          )}
        </div>
      )}
    </div>
  );
}
