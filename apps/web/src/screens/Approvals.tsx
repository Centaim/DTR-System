import { useCallback, useEffect, useState } from "react";
import { Check, X } from "lucide-react";
import { ApiError, NetworkError, api, type RequestItem } from "../lib/api";
import { useT, type Strings } from "../lib/i18n";
import { describe, kindLabel } from "./Requests";

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

  const load = useCallback(() => {
    setFailed(false);
    api.approvals
      .pending()
      .then((l) => {
        setList(l);
        onCount(l.length);
      })
      .catch(() => setFailed(true));
  }, [onCount]);
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
