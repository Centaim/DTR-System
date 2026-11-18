import { useCallback, useEffect, useState, type FormEvent } from "react";
import { ApiError, api, type Accomplishment, type Employee } from "../lib/api";
import { useT, type Strings } from "../lib/i18n";

const today = () => new Intl.DateTimeFormat("en-CA", { timeZone: "Asia/Manila" }).format(new Date());
const st = (t: Strings, s: string) => t[`accSt_${s}` as keyof Strings] as string;

export function Accomplishments({ employee }: { employee: Employee }) {
  const { t } = useT();
  const approver = ["supervisor", "hr_admin", "super_admin"].includes(employee.role);
  const [mine, setMine] = useState<Accomplishment[] | null>(null);
  const [pending, setPending] = useState<Accomplishment[]>([]);
  const [failed, setFailed] = useState(false);
  const [f, setF] = useState({ period_from: today(), period_to: today(), body: "" });
  const [error, setError] = useState("");
  const [ok, setOk] = useState(false);
  const [returning, setReturning] = useState<number | null>(null);
  const [note, setNote] = useState("");

  const load = useCallback(() => {
    setFailed(false);
    api.accomplishments.mine().then(setMine).catch(() => setFailed(true));
    if (approver) api.accomplishments.pending().then(setPending).catch(() => setFailed(true));
  }, [approver]);
  useEffect(load, [load]);

  const submit = async (ev: FormEvent) => {
    ev.preventDefault();
    setError("");
    setOk(false);
    try {
      await api.accomplishments.create(f);
      setF({ ...f, body: "" });
      setOk(true);
      load();
    } catch (e) {
      setError(e instanceof ApiError && e.message ? e.message : t.errGeneric);
    }
  };

  const review = async (id: number, accept: boolean) => {
    setError("");
    try {
      await api.accomplishments.review(id, accept, accept ? "" : note);
      setReturning(null);
      setNote("");
      load();
    } catch (e) {
      setError(e instanceof ApiError && e.message ? e.message : t.errGeneric);
    }
  };

  return (
    <div className="shell">
      <header className="topbar"><span className="brand">{t.accTitle}</span></header>
      <main className="main admin-main with-nav">
        {failed && <p className="msg bad" role="alert">{t.loadFailed} <button className="link-btn" onClick={load}>{t.retry}</button></p>}
        <div className="admin-cols">
          <form className="panel form" onSubmit={submit} noValidate>
            <h1 className="panel-h">{t.accNew}</h1>
            <div className="two">
              <label className="field"><span>{t.accFrom}</span><input type="date" value={f.period_from} onChange={(e) => setF({ ...f, period_from: e.target.value })} /></label>
              <label className="field"><span>{t.accTo}</span><input type="date" value={f.period_to} min={f.period_from} onChange={(e) => setF({ ...f, period_to: e.target.value })} /></label>
            </div>
            <label className="field"><span>{t.accBody}</span><textarea rows={6} maxLength={8000} value={f.body} onChange={(e) => setF({ ...f, body: e.target.value })} /></label>
            <div aria-live="polite">
              {error && <p className="msg bad">{error}</p>}
              {ok && <p className="msg ok">{t.accSaved}</p>}
            </div>
            <button className="btn primary" type="submit" disabled={f.body.trim().length < 10}>{t.accSubmit}</button>
          </form>

          <div className="stack">
            {approver && (
              <section className="panel" aria-labelledby="ar-h">
                <h2 id="ar-h" className="panel-h">{t.accReview}</h2>
                {pending.length === 0 && <p className="muted">{t.accReviewEmpty}</p>}
                <ul className="rows flat">
                  {pending.map((a) => (
                    <li key={a.id} className="row approval">
                      <span className="row-main">
                        <span className="rail-slot">{a.employee}</span>
                        <span className="rail-state">{a.period_from} – {a.period_to}</span>
                        <span className="pre">{a.body}</span>
                        {returning === a.id ? (
                          <span className="reject-box">
                            <label className="field"><span>{t.accReturnWhy}</span><textarea rows={2} maxLength={300} value={note} onChange={(e) => setNote(e.target.value)} /></label>
                            <span className="actions">
                              <button className="btn primary" disabled={note.trim().length < 3} onClick={() => review(a.id, false)}>{t.accReturn}</button>
                              <button className="btn quiet" onClick={() => setReturning(null)}>{t.cancel}</button>
                            </span>
                          </span>
                        ) : (
                          <span className="actions">
                            <button className="btn primary" onClick={() => review(a.id, true)}>{t.accAccept}</button>
                            <button className="btn quiet" onClick={() => { setReturning(a.id); setNote(""); }}>{t.accReturn}</button>
                          </span>
                        )}
                      </span>
                    </li>
                  ))}
                </ul>
              </section>
            )}
            <section className="panel" aria-labelledby="am-h">
              <h2 id="am-h" className="panel-h">{t.accMine}</h2>
              {mine?.length === 0 && <p className="muted">{t.accEmpty}</p>}
              <ul className="rows flat">
                {mine?.map((a) => (
                  <li key={a.id} className="row">
                    <span className="row-main">
                      <span className="rail-slot">{a.period_from} – {a.period_to}</span>
                      <span className="pre">{a.body}</span>
                      {a.note && <span className="rail-state">{t.decisionNoteShown(a.note)}</span>}
                    </span>
                    <span className="row-side"><span className={`tag ${a.status === "accepted" ? "status-approved" : a.status === "returned" ? "status-rejected" : "status-pending"}`}>{st(t, a.status)}</span></span>
                  </li>
                ))}
              </ul>
            </section>
          </div>
        </div>
      </main>
    </div>
  );
}
