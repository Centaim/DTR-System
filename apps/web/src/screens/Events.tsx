import { useCallback, useEffect, useState, type FormEvent } from "react";
import { ApiError, api, type Attendee, type EventItem, type Employee } from "../lib/api";
import { useT } from "../lib/i18n";
import { formatTime } from "../lib/time";
import { CameraSheet } from "../components/CameraSheet";

const when = (iso: string) =>
  new Intl.DateTimeFormat("en-PH", { timeZone: "Asia/Manila", weekday: "short", month: "short", day: "numeric" }).format(new Date(iso)) + ", " + formatTime(iso);

export function Events({ employee }: { employee: Employee }) {
  const { t } = useT();
  const staff = ["hr_admin", "super_admin", "timekeeper"].includes(employee.role);
  const viewer = staff || employee.role === "auditor";
  const [list, setList] = useState<EventItem[] | null>(null);
  const [failed, setFailed] = useState(false);
  const [camera, setCamera] = useState<number | null>(null);
  const [roster, setRoster] = useState<{ id: number; rows: Attendee[] } | null>(null);
  const [markNo, setMarkNo] = useState("");
  const [error, setError] = useState("");
  const [f, setF] = useState({ title: "", place: "", office: "", starts_at: "", ends_at: "", required: true });

  const fail = (e: unknown) => setError(e instanceof ApiError && e.message ? e.message : t.errGeneric);
  const load = useCallback(() => {
    setFailed(false);
    api.events.list().then(setList).catch(() => setFailed(true));
  }, []);
  useEffect(load, [load]);

  const openRoster = (id: number) => api.events.attendance(id).then((rows) => setRoster({ id, rows })).catch(fail);

  const checkIn = async (photo: Blob) => {
    const id = camera;
    setCamera(null);
    if (id === null) return;
    setError("");
    try {
      await api.events.checkIn(id, photo);
      load();
    } catch (e) {
      fail(e);
    }
  };

  const create = async (ev: FormEvent) => {
    ev.preventDefault();
    setError("");
    try {
      await api.events.create(f);
      setF({ title: "", place: "", office: "", starts_at: "", ends_at: "", required: true });
      load();
    } catch (e) {
      fail(e);
    }
  };

  const mark = async (id: number) => {
    setError("");
    try {
      await api.events.mark(id, markNo);
      setMarkNo("");
      await openRoster(id);
      load();
    } catch (e) {
      fail(e);
    }
  };

  const now = Date.now();
  return (
    <div className="shell">
      <header className="topbar"><span className="brand">{t.evTitle}</span></header>
      <main className="main admin-main with-nav">
        {failed && <p className="msg bad" role="alert">{t.loadFailed} <button className="link-btn" onClick={load}>{t.retry}</button></p>}
        <div aria-live="polite">{error && <p className="msg bad">{error}</p>}</div>
        <div className="admin-cols">
          <section className="panel" aria-labelledby="ev-h">
            <h1 id="ev-h" className="panel-h">{t.evTitle}</h1>
            {list?.length === 0 && <p className="muted">{t.evEmpty}</p>}
            <ul className="rows flat">
              {list?.map((e) => {
                const open = now >= Date.parse(e.starts_at) - 30 * 60000 && now <= Date.parse(e.ends_at);
                return (
                  <li key={e.id} className="row approval">
                    <span className="row-main">
                      <span className="rail-slot">{e.title} {e.required && <span className="tag">{t.evRequired}</span>}</span>
                      <span className="rail-state">{when(e.starts_at)}{e.place ? ` · ${e.place}` : ""}</span>
                      {viewer && <span className="rail-state">{t.evCount(e.count, e.audience)}</span>}
                      {e.attended ? (
                        <span className="tag status-approved fit">{t.evCheckedIn}</span>
                      ) : open ? (
                        <span className="actions"><button className="btn primary" onClick={() => setCamera(e.id)}>{t.evCheckIn}</button></span>
                      ) : (
                        now > Date.parse(e.ends_at) && <span className="rail-state">{t.evClosed}</span>
                      )}
                      {viewer && (
                        <button className="link-btn start" onClick={() => (roster?.id === e.id ? setRoster(null) : openRoster(e.id))}>
                          {roster?.id === e.id ? t.evHide : t.evRoster}
                        </button>
                      )}
                      {roster?.id === e.id && (
                        <span className="reject-box">
                          {staff && (
                            <span className="inline-add">
                              <label className="field"><span>{t.evMarkNo}</span><input value={markNo} onChange={(x) => setMarkNo(x.target.value)} /></label>
                              <button className="btn quiet" disabled={!markNo.trim()} onClick={() => mark(e.id)}>{t.evMark}</button>
                            </span>
                          )}
                          <ul className="rows flat">
                            {roster.rows.map((r) => (
                              <li key={r.employee_no} className="row">
                                <span className="row-main"><span className="rail-slot">{r.name}</span><span className="rail-state">{r.employee_no}</span></span>
                                <span className="rail-state">{r.at ? formatTime(r.at) : t.evAbsent}</span>
                              </li>
                            ))}
                          </ul>
                        </span>
                      )}
                    </span>
                  </li>
                );
              })}
            </ul>
          </section>

          {staff && (
            <form className="panel form" onSubmit={create} noValidate>
              <h2 className="panel-h">{t.evCreate}</h2>
              <label className="field"><span>{t.evName}</span><input value={f.title} maxLength={120} onChange={(e) => setF({ ...f, title: e.target.value })} /></label>
              <label className="field"><span>{t.evPlace}</span><input value={f.place} maxLength={120} onChange={(e) => setF({ ...f, place: e.target.value })} /></label>
              <label className="field"><span>{t.evOffice}</span><input value={f.office} maxLength={80} onChange={(e) => setF({ ...f, office: e.target.value })} /></label>
              <label className="field"><span>{t.evStarts}</span><input type="datetime-local" value={f.starts_at} onChange={(e) => setF({ ...f, starts_at: e.target.value })} /></label>
              <label className="field"><span>{t.evEnds}</span><input type="datetime-local" value={f.ends_at} onChange={(e) => setF({ ...f, ends_at: e.target.value })} /></label>
              <label className="pick"><input type="checkbox" checked={f.required} onChange={(e) => setF({ ...f, required: e.target.checked })} /><span>{t.evIsRequired}</span></label>
              <button className="btn primary" type="submit" disabled={f.title.trim().length < 3 || !f.starts_at || !f.ends_at}>{t.evCreate}</button>
            </form>
          )}
        </div>
      </main>
      {camera !== null && <CameraSheet onCancel={() => setCamera(null)} onConfirm={checkIn} />}
    </div>
  );
}
