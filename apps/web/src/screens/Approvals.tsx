import { useCallback, useEffect, useState } from "react";
import { Check, X } from "lucide-react";
import { ApiError, NetworkError, api, type DtrPendingRow, type DtrResponse, type RequestItem } from "../lib/api";
import { useT, type Strings } from "../lib/i18n";
import { describe, kindLabel } from "./Requests";
import { monthLabel } from "../lib/month";
import { Form48 } from "../components/Form48";

function message(e: unknown, t: Strings): string {
  if (e instanceof NetworkError) return t.errNetwork;
  if (e instanceof ApiError && e.message) return e.message;
  return t.errGeneric;
}

export function Approvals({ onCount }: { onCount: (n: number) => void }) {
  const { t, lang } = useT();
  const [list, setList] = useState<RequestItem[] | null>(null);
  const [failed, setFailed] = useState(false);
  const [rejecting, setRejecting] = useState<number | null>(null);
  const [note, setNote] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");

  const [dtrs, setDtrs] = useState<DtrPendingRow[]>([]);
  const [open, setOpen] = useState<{ id: number; dtr: Pick<DtrResponse, "employee" | "sheet" | "official_hours"> } | null>(null);
  const [returning, setReturning] = useState(false);

  const load = useCallback(() => {
    setFailed(false);
    Promise.all([api.approvals.pending(), api.dtrReview.pending()])
      .then(([l, d]) => {
        setList(l);
        setDtrs(d);
        onCount(l.length + d.length);
      })
      .catch(() => setFailed(true));
  }, [onCount]);

  const openDtr = async (id: number) => {
    setError("");
    try {
      const r = await api.dtrReview.open(id);
      setOpen({ id, dtr: r.dtr });
      setReturning(false);
      setNote("");
    } catch (e) {
      setError(message(e, t));
    }
  };

  const decideDtr = async (verify: boolean) => {
    if (!open) return;
    setBusy(true);
    setError("");
    try {
      if (verify) await api.dtrReview.verify(open.id);
      else await api.dtrReview.reject(open.id, note);
      setOpen(null);
      setNote("");
    } catch (e) {
      setError(message(e, t));
    } finally {
      setBusy(false);
      load();
    }
  };
  useEffect(load, [load]);

  const decide = async (id: number, approve: boolean) => {
    setBusy(true);
    setError("");
    try {
      await api.approvals.decide(id, approve, approve ? "" : note);
      setRejecting(null);
      setNote("");
    } catch (e) {
      setError(message(e, t));
    } finally {
      setBusy(false);
      load();
    }
  };

  return (
    <div className="shell">
      <header className="topbar">
        <span className="brand">{t.approvalsTitle}</span>
      </header>
      <main className="main admin-main with-nav">
        <section className="panel" aria-labelledby="dv-h">
          <h1 id="dv-h" className="panel-h">{t.dtrToVerify}</h1>
          {dtrs.length === 0 && !open && <p className="muted">{t.dtrNoneToVerify}</p>}
          {open ? (
            <div className="form">
              <div className="paper-scroll"><Form48 data={open.dtr} /></div>
              {returning ? (
                <span className="reject-box">
                  <label className="field">
                    <span>{t.dtrReturnWhy}</span>
                    <textarea value={note} onChange={(e) => setNote(e.target.value)} rows={2} maxLength={300} />
                  </label>
                  <span className="actions">
                    <button className="btn primary" disabled={busy || note.trim().length < 3} onClick={() => decideDtr(false)}>{t.dtrConfirmReturn}</button>
                    <button className="btn quiet" onClick={() => setReturning(false)}>{t.cancel}</button>
                  </span>
                </span>
              ) : (
                <span className="actions">
                  <button className="btn primary" disabled={busy} onClick={() => decideDtr(true)}><Check aria-hidden="true" size={20} /> {t.dtrVerify}</button>
                  <button className="btn quiet" disabled={busy} onClick={() => setReturning(true)}><X aria-hidden="true" size={20} /> {t.dtrReturn}</button>
                </span>
              )}
              <button className="link-btn start" onClick={() => setOpen(null)}>{t.back}</button>
            </div>
          ) : (
            <ul className="rows flat">
              {dtrs.map((d) => (
                <li key={d.id} className="row">
                  <span className="row-main">
                    <span className="rail-slot">{d.name}</span>
                    <span className="rail-state">{monthLabel(d.month)} · {t.dtrVersion(d.version)}</span>
                  </span>
                  <button className="link-btn" onClick={() => openDtr(d.id)}>{t.dtrOpen}</button>
                </li>
              ))}
            </ul>
          )}
        </section>

        <section className="panel" aria-labelledby="ap-h">
          <h1 id="ap-h" className="panel-h">
            {t.approvalsTitle}
          </h1>
          {failed && (
            <p className="msg bad" role="alert">
              {t.loadFailed} <button className="link-btn" onClick={load}>{t.retry}</button>
            </p>
          )}
          {list && list.length === 0 && <p className="muted">{t.noApprovals}</p>}
          <div aria-live="polite">{error && <p className="msg bad">{error}</p>}</div>
          <ul className="rows">
            {list?.map((r) => (
              <li key={r.id} className="row approval">
                <span className="row-main">
                  <span className="rail-slot">
                    {r.employee} <span className="rail-state">{r.employee_no}</span>
                  </span>
                  <span>{kindLabel(t, r.kind)}</span>
                  <span className="rail-state">{describe(t, lang, r)}</span>
                  <span className="rail-state">{r.reason}</span>

                  {rejecting === r.id ? (
                    <span className="reject-box">
                      <label className="field">
                        <span>{t.decisionNote}</span>
                        <textarea value={note} onChange={(e) => setNote(e.target.value)} rows={2} maxLength={300} />
                      </label>
                      <span className="actions">
                        <button className="btn primary" disabled={busy || note.trim().length < 3} onClick={() => decide(r.id, false)}>
                          {t.confirmReject}
                        </button>
                        <button className="btn quiet" onClick={() => setRejecting(null)}>
                          {t.cancel}
                        </button>
                      </span>
                    </span>
                  ) : (
                    <span className="actions">
                      <button className="btn primary" disabled={busy} onClick={() => decide(r.id, true)}>
                        <Check aria-hidden="true" size={20} /> {t.approve}
                      </button>
                      <button className="btn quiet" disabled={busy} onClick={() => { setRejecting(r.id); setNote(""); }}>
                        <X aria-hidden="true" size={20} /> {t.reject}
                      </button>
                    </span>
                  )}
                </span>
              </li>
            ))}
          </ul>
        </section>
      </main>
    </div>
  );
}
