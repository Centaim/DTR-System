import { useState, type FormEvent } from "react";
import { ApiError, api } from "../lib/api";
import { slotLabel, useT } from "../lib/i18n";

const SLOTS = ["am_in", "am_out", "pm_in", "pm_out"] as const;

export function Logbook() {
  const { t } = useT();
  const [f, setF] = useState({ employee_no: "", day: "", reason: "" });
  const [times, setTimes] = useState<Record<string, string>>({});
  const [error, setError] = useState("");
  const [saved, setSaved] = useState<number | null>(null);
  const entries = SLOTS.filter((s) => times[s]).map((s) => ({ slot: s, time: times[s]! }));

  const save = async (ev: FormEvent) => {
    ev.preventDefault();
    setError("");
    setSaved(null);
    try {
      const r = await api.logbook({ ...f, entries });
      setSaved(r.saved);
      setTimes({});
      setF({ ...f, employee_no: "" });
    } catch (e) {
      setError(e instanceof ApiError && e.message ? e.message : t.errGeneric);
    }
  };

  return (
    <div className="shell">
      <header className="topbar"><span className="brand">{t.logTitle}</span></header>
      <main className="main narrow with-nav">
        <form className="panel form" onSubmit={save} noValidate>
          <h1 className="panel-h">{t.logTitle}</h1>
          <p className="muted small">{t.logNote}</p>
          <label className="field"><span>{t.logEmployee}</span><input value={f.employee_no} onChange={(e) => setF({ ...f, employee_no: e.target.value })} /></label>
          <label className="field"><span>{t.logDate}</span><input type="date" value={f.day} onChange={(e) => setF({ ...f, day: e.target.value })} /></label>
          <div className="two">
            {SLOTS.map((s) => (
              <label key={s} className="field">
                <span>{slotLabel(t, s)}</span>
                <input type="time" value={times[s] ?? ""} onChange={(e) => setTimes({ ...times, [s]: e.target.value })} />
              </label>
            ))}
          </div>
          <label className="field"><span>{t.logReason}</span><textarea rows={2} maxLength={500} value={f.reason} onChange={(e) => setF({ ...f, reason: e.target.value })} /></label>
          <div aria-live="polite">
            {error && <p className="msg bad">{error}</p>}
            {saved !== null && <p className="msg ok">{t.logSaved(saved)}</p>}
          </div>
          <button className="btn primary" type="submit" disabled={!f.employee_no.trim() || !f.day || entries.length === 0 || f.reason.trim().length < 5}>{t.logSave}</button>
        </form>
      </main>
    </div>
  );
}
