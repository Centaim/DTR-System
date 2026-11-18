import { useCallback, useEffect, useState, type FormEvent } from "react";
import { ApiError, api, type Schedule } from "../../lib/api";
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
  });
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");

  const load = useCallback(() => {
    setFailed(false);
    api.admin.schedules().then(setList).catch(() => setFailed(true));
  }, []);
  useEffect(load, [load]);

  const submit = async (ev: FormEvent) => {
    ev.preventDefault();
    setBusy(true);
    setError("");
    try {
      await api.admin.createSchedule(form);
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
        <div aria-live="polite">{error && <p className="msg bad">{error}</p>}</div>
        <button className="btn primary" type="submit" disabled={busy || !form.name.trim()}>
          {busy ? t.saving : t.createSchedule}
        </button>
      </form>
    </div>
  );
}
