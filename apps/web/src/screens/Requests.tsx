import { useCallback, useEffect, useState, type FormEvent } from "react";
import { ApiError, NetworkError, api, type RequestItem } from "../lib/api";
import { slotLabel, useT, type Strings } from "../lib/i18n";
import { formatDate } from "../lib/time";

export const KINDS = ["missed_log", "official_business", "travel_order", "leave"] as const;
const SLOTS = ["am_in", "am_out", "pm_in", "pm_out"] as const;

const todayStr = () => new Intl.DateTimeFormat("en-CA", { timeZone: "Asia/Manila" }).format(new Date());
export const kindLabel = (t: Strings, k: string) => (t[`kind_${k}` as keyof Strings] as string) ?? k;
export const statusLabel = (t: Strings, s: string) => (t[`status_${s}` as keyof Strings] as string) ?? s;

const dayText = (iso: string, lang: string) => formatDate(Date.parse(`${iso}T12:00:00+08:00`), lang);

export function describe(t: Strings, lang: string, r: RequestItem): string {
  const days = r.date_to && r.date_to !== r.work_date
    ? `${dayText(r.work_date, lang)} – ${dayText(r.date_to, lang)}`
    : dayText(r.work_date, lang);
  if (r.kind === "missed_log" && r.slot && r.time) return `${days} · ${slotLabel(t, r.slot)} ${r.time}`;
  return days;
}

function message(e: unknown, t: Strings): string {
  if (e instanceof NetworkError) return t.errNetwork;
  if (e instanceof ApiError && e.message) return e.message;
  return t.errGeneric;
}

export function Requests() {
  const { t, lang } = useT();
  const [list, setList] = useState<RequestItem[] | null>(null);
  const [failed, setFailed] = useState(false);
  const [f, setF] = useState({ kind: "missed_log", work_date: todayStr(), date_to: "", slot: "am_in", time: "08:00", reason: "" });
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const [ok, setOk] = useState(false);
  const [file, setFile] = useState<File | null>(null);

  const load = useCallback(() => {
    setFailed(false);
    api.requests.mine().then(setList).catch(() => setFailed(true));
  }, []);
  useEffect(load, [load]);

  const missed = f.kind === "missed_log";

  const submit = async (ev: FormEvent) => {
    ev.preventDefault();
    setBusy(true);
    setError("");
    setOk(false);
    try {
      const made = await api.requests.create({
        kind: f.kind,
        work_date: f.work_date,
        ...(missed ? { slot: f.slot, time: f.time } : f.date_to ? { date_to: f.date_to } : {}),
        reason: f.reason,
      });
      if (file) await api.attachment.upload(made.id, file);
      setFile(null);
      setF({ ...f, reason: "" });
      setOk(true);
      load();
    } catch (e) {
      setError(message(e, t));
    } finally {
      setBusy(false);
    }
  };

  const cancel = async (id: number) => {
    try {
      await api.requests.cancel(id);
    } catch (e) {
      setError(message(e, t));
    }
    load();
  };

  return (
    <div className="shell">
      <header className="topbar">
        <span className="brand">{t.reqTitle}</span>
      </header>
      <main className="main admin-main with-nav">
        <div className="admin-cols">
          <form className="panel form" onSubmit={submit} noValidate>
            <h1 className="panel-h">{t.newRequest}</h1>
            <label className="field">
              <span>{t.reqKind}</span>
              <select value={f.kind} onChange={(e) => setF({ ...f, kind: e.target.value })}>
                {KINDS.map((k) => (
                  <option key={k} value={k}>
                    {kindLabel(t, k)}
                  </option>
                ))}
              </select>
            </label>
            <label className="field">
              <span>{t.reqDate}</span>
              <input type="date" value={f.work_date} onChange={(e) => setF({ ...f, work_date: e.target.value })} required />
            </label>
            {missed ? (
              <>
                <label className="field">
                  <span>{t.reqSlot}</span>
                  <select value={f.slot} onChange={(e) => setF({ ...f, slot: e.target.value })}>
                    {SLOTS.map((s) => (
                      <option key={s} value={s}>
                        {slotLabel(t, s)}
                      </option>
                    ))}
                  </select>
                </label>
                <label className="field">
                  <span>{t.reqTime}</span>
                  <input type="time" value={f.time} onChange={(e) => setF({ ...f, time: e.target.value })} required />
                </label>
              </>
            ) : (
              <label className="field">
                <span>{t.reqDateTo}</span>
                <input type="date" value={f.date_to} min={f.work_date} onChange={(e) => setF({ ...f, date_to: e.target.value })} />
              </label>
            )}
            <label className="field">
              <span>{t.reqReason}</span>
              <textarea value={f.reason} onChange={(e) => setF({ ...f, reason: e.target.value })} rows={3} maxLength={500} required />
            </label>
            <label className="field">
              <span>{t.reqAttach}</span>
              <input type="file" accept="image/jpeg,image/png,application/pdf" onChange={(e) => setFile(e.target.files?.[0] ?? null)} />
              <small className="muted">{t.reqAttachHelp}</small>
            </label>
            <div aria-live="polite">
              {error && <p className="msg bad">{error}</p>}
              {ok && <p className="msg ok">{t.requestSent}</p>}
            </div>
            <button className="btn primary" type="submit" disabled={busy || f.reason.trim().length < 5}>
              {busy ? t.sending : t.submitRequest}
            </button>
          </form>

          <section className="panel" aria-labelledby="mine-h">
            <h2 id="mine-h" className="panel-h">
              {t.yourRequests}
            </h2>
            {failed && (
              <p className="msg bad" role="alert">
                {t.loadFailed} <button className="link-btn" onClick={load}>{t.retry}</button>
              </p>
            )}
            {list && list.length === 0 && <p className="muted">{t.noRequests}</p>}
            <ul className="rows">
              {list?.map((r) => (
                <li key={r.id} className="row">
                  <span className="row-main">
                    <span className="rail-slot">{kindLabel(t, r.kind)}</span>
                    <span className="rail-state">{describe(t, lang, r)}</span>
                    <span className="rail-state">{r.reason}</span>
                    {r.decision_note && <span className="rail-state">{t.decisionNoteShown(r.decision_note)}</span>}
                    {r.status === "pending" && (
                      <button className="link-btn start" onClick={() => cancel(r.id)}>
                        {t.cancelRequest}
                      </button>
                    )}
                  </span>
                  <span className="row-side">
                    <span className={`tag status-${r.status}`}>{statusLabel(t, r.status)}</span>
                  </span>
                </li>
              ))}
            </ul>
          </section>
        </div>
      </main>
    </div>
  );
}
