import { useCallback, useEffect, useState, type FormEvent } from "react";
import { ApiError, api, type Holiday, type Schedule, type Suspension } from "../../lib/api";
import { useT } from "../../lib/i18n";

const to12 = (hhmm: string) => {
  const [h = "0", m = "00"] = hhmm.split(":");
  return `${Number(h) % 12 || 12}:${m} ${Number(h) < 12 ? "AM" : "PM"}`;
};

export function SchedulesTab() {
  const { t } = useT();
  const [list, setList] = useState<Schedule[] | null>(null);
  const [failed, setFailed] = useState(false);
  const [form, setForm] = useState({
    name: "",
    am_start: "08:00",
    am_end: "12:00",
    pm_start: "13:00",
    pm_end: "17:00",
    flexi: false,
    grace_minutes: 0,
    required_hours: 8,
    latest_in: "09:30",
    work_days: 31,
    two_logs: false,
    overnight: false,
  });
  const [suspensions, setSuspensions] = useState<Suspension[]>([]);
  const [susp, setSusp] = useState({ day: "", from_time: "", office: "", reason: "" });
  const [suspError, setSuspError] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");

  const year = new Date().getFullYear();
  const [holidays, setHolidays] = useState<Holiday[]>([]);
  const [hol, setHol] = useState({ day: "", name: "" });
  const [holError, setHolError] = useState("");

  const load = useCallback(() => {
    setFailed(false);
    api.admin.schedules().then(setList).catch(() => setFailed(true));
    api.reports.holidays(year).then(setHolidays).catch(() => undefined);
    api.admin.suspensions().then(setSuspensions).catch(() => undefined);
  }, [year]);
  useEffect(load, [load]);

  const addHoliday = async (ev: FormEvent) => {
    ev.preventDefault();
    setHolError("");
    try {
      await api.admin.createHoliday(hol);
      setHol({ day: "", name: "" });
      load();
    } catch (e) {
      setHolError(e instanceof ApiError && e.message ? e.message : t.errGeneric);
    }
  };

  const addSuspension = async (ev: FormEvent) => {
    ev.preventDefault();
    setSuspError("");
    try {
      await api.admin.createSuspension(susp);
      setSusp({ day: "", from_time: "", office: "", reason: "" });
      load();
    } catch (e) {
      setSuspError(e instanceof ApiError && e.message ? e.message : t.errGeneric);
    }
  };

  const summary = (s: Schedule) =>
    [
      s.flexi ? t.schedSummaryFlexi(s.required_minutes / 60) : "",
      s.grace_minutes ? t.schedSummaryGrace(s.grace_minutes) : "",
      s.two_logs ? t.schedSummaryTwo : "",
      s.overnight ? t.schedSummaryNight : "",
      t.dayNames.split(",").filter((_, i) => s.work_days & (1 << i)).join(" "),
    ].filter(Boolean).join(" · ");

  const removeHoliday = async (day: string) => {
    try {
      await api.admin.deleteHoliday(day);
    } catch {
      /* the list reload shows the true state */
    }
    load();
  };

  const submit = async (ev: FormEvent) => {
    ev.preventDefault();
    setBusy(true);
    setError("");
    try {
      const { required_hours, ...rest } = form;
      await api.admin.createSchedule({ ...rest, required_minutes: Math.round(required_hours * 60) });
      setForm({ ...form, name: "" });
      load();
    } catch (e) {
      setError(e instanceof ApiError && e.message ? e.message : t.errGeneric);
    } finally {
      setBusy(false);
    }
  };

  const time = (key: "am_start" | "am_end" | "pm_start" | "pm_end", label: string) => (
    <label className="field">
      <span>{label}</span>
      <input type="time" value={form[key]} onChange={(e) => setForm({ ...form, [key]: e.target.value })} required />
    </label>
  );

  const time2 = (key: "latest_in", label: string) => (
    <label className="field">
      <span>{label}</span>
      <input type="time" value={form[key]} onChange={(e) => setForm({ ...form, [key]: e.target.value })} />
    </label>
  );

  return (
    <div className="admin-cols">
      <section className="panel" aria-labelledby="sch-h">
        <h2 id="sch-h" className="panel-h">
          {t.tabSchedules}
        </h2>
        <p className="muted small">{t.scheduleHint}</p>
        {failed && (
          <p className="msg bad" role="alert">
            {t.loadFailed} <button className="link-btn" onClick={load}>{t.retry}</button>
          </p>
        )}
        <ul className="rows">
          {list?.map((s) => (
            <li key={s.id} className="row">
              <span className="row-main">
                <span className="rail-slot">
                  {s.name} {s.is_default && <span className="tag">{t.defaultTag}</span>}
                </span>
                <span className="rail-state">
                  {to12(s.am_start)} – {to12(s.am_end)}, {to12(s.pm_start)} – {to12(s.pm_end)}
                </span>
                <span className="rail-state">{summary(s)}</span>
              </span>
            </li>
          ))}
        </ul>
      </section>

      <form className="panel form" onSubmit={submit} noValidate>
        <h2 className="panel-h">{t.createSchedule}</h2>
        <label className="field">
          <span>{t.scheduleName}</span>
          <input value={form.name} onChange={(e) => setForm({ ...form, name: e.target.value })} maxLength={60} required />
        </label>
        <div className="two">
          {time("am_start", t.amStart)}
          {time("am_end", t.amEnd)}
          {time("pm_start", t.pmStart)}
          {time("pm_end", t.pmEnd)}
        </div>
        <label className="field">
          <span>{t.schedType}</span>
          <select value={form.flexi ? "flexi" : "fixed"} onChange={(e) => setForm({ ...form, flexi: e.target.value === "flexi" })}>
            <option value="fixed">{t.schedFixed}</option>
            <option value="flexi">{t.schedFlexi}</option>
          </select>
        </label>
        {form.flexi ? (
          <div className="two">
            <label className="field">
              <span>{t.schedRequired}</span>
              <input type="number" min={1} max={24} step={0.5} value={form.required_hours} onChange={(e) => setForm({ ...form, required_hours: Number(e.target.value) })} />
            </label>
            {time2("latest_in", t.schedLatest)}
          </div>
        ) : (
          <label className="field">
            <span>{t.schedGrace}</span>
            <input type="number" min={0} max={120} value={form.grace_minutes} onChange={(e) => setForm({ ...form, grace_minutes: Number(e.target.value) })} />
          </label>
        )}
        <fieldset className="set-group">
          <legend>{t.schedDays}</legend>
          <div className="day-picks">
            {t.dayNames.split(",").map((d, i) => (
              <label key={d} className="day-pick">
                <input type="checkbox" checked={(form.work_days & (1 << i)) !== 0} onChange={() => setForm({ ...form, work_days: form.work_days ^ (1 << i) })} />
                <span>{d}</span>
              </label>
            ))}
          </div>
        </fieldset>
        <label className="pick">
          <input type="checkbox" checked={form.two_logs} onChange={(e) => setForm({ ...form, two_logs: e.target.checked })} />
          <span>{t.schedTwoLogs}</span>
        </label>
        <label className="pick">
          <input type="checkbox" checked={form.overnight} onChange={(e) => setForm({ ...form, overnight: e.target.checked })} />
          <span>{t.schedOvernight}</span>
        </label>
        <div aria-live="polite">{error && <p className="msg bad">{error}</p>}</div>
        <button className="btn primary" type="submit" disabled={busy || !form.name.trim() || form.work_days === 0}>
          {busy ? t.saving : t.createSchedule}
        </button>
      </form>

      <section className="panel" aria-labelledby="hol-h">
        <h2 id="hol-h" className="panel-h">
          {t.holidaysHeading} {year}
        </h2>
        <p className="muted small">{t.holidayNote}</p>
        {holidays.length === 0 && <p className="muted">{t.noHolidays}</p>}
        <ul className="rows flat">
          {holidays.map((h) => (
            <li key={h.day} className="row">
              <span className="row-main">
                <span className="rail-slot">{h.name}</span>
                <span className="rail-state">{h.day}</span>
              </span>
              <button className="link-btn" onClick={() => removeHoliday(h.day)}>
                {t.removeHoliday}
              </button>
            </li>
          ))}
        </ul>
      </section>

      <section className="panel" aria-labelledby="susp-h">
        <h2 id="susp-h" className="panel-h">{t.suspTitle}</h2>
        <p className="muted small">{t.suspNote}</p>
        {suspensions.length === 0 && <p className="muted">{t.suspEmpty}</p>}
        <ul className="rows flat">
          {suspensions.map((x) => (
            <li key={x.id} className="row">
              <span className="row-main">
                <span className="rail-slot">{x.day} · {x.from_time ? t.suspFromTime(to12(x.from_time)) : t.suspWhole}</span>
                <span className="rail-state">{x.office || t.suspAll} · {x.reason}</span>
              </span>
              <button className="link-btn" onClick={() => api.admin.deleteSuspension(x.id).finally(load)}>{t.removeHoliday}</button>
            </li>
          ))}
        </ul>
      </section>

      <form className="panel form" onSubmit={addSuspension} noValidate>
        <h2 className="panel-h">{t.suspAdd}</h2>
        <label className="field">
          <span>{t.suspDate}</span>
          <input type="date" value={susp.day} onChange={(e) => setSusp({ ...susp, day: e.target.value })} />
        </label>
        <label className="field">
          <span>{t.suspFrom}</span>
          <input type="time" value={susp.from_time} onChange={(e) => setSusp({ ...susp, from_time: e.target.value })} />
        </label>
        <label className="field">
          <span>{t.suspOffice}</span>
          <input value={susp.office} maxLength={80} onChange={(e) => setSusp({ ...susp, office: e.target.value })} />
        </label>
        <label className="field">
          <span>{t.suspReason}</span>
          <input value={susp.reason} maxLength={200} onChange={(e) => setSusp({ ...susp, reason: e.target.value })} />
        </label>
        <div aria-live="polite">{suspError && <p className="msg bad">{suspError}</p>}</div>
        <button className="btn primary" type="submit" disabled={!susp.day || susp.reason.trim().length < 3}>{t.suspAdd}</button>
      </form>

      <form className="panel form" onSubmit={addHoliday} noValidate>
        <h2 className="panel-h">{t.addHoliday}</h2>
        <label className="field">
          <span>{t.holidayDate}</span>
          <input type="date" value={hol.day} onChange={(e) => setHol({ ...hol, day: e.target.value })} required />
        </label>
        <label className="field">
          <span>{t.holidayName}</span>
          <input value={hol.name} onChange={(e) => setHol({ ...hol, name: e.target.value })} maxLength={80} required />
        </label>
        <div aria-live="polite">{holError && <p className="msg bad">{holError}</p>}</div>
        <button className="btn primary" type="submit" disabled={!hol.day || !hol.name.trim()}>
          {t.addHoliday}
        </button>
      </form>
    </div>
  );
}
