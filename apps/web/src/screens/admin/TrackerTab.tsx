import { useCallback, useEffect, useMemo, useState } from "react";
import { ChevronLeft, ChevronRight, Printer } from "lucide-react";
import { ApiError, api, type DtrResponse, type TrackerRow } from "../../lib/api";
import { useT, type Strings } from "../../lib/i18n";
import { currentMonth, monthLabel, shiftMonth } from "../../lib/month";
import { Form48 } from "../../components/Form48";

const stLabel = (t: Strings, s: string) => t[`dtrSt_${s}` as keyof Strings] as string;
const tone = (s: string) => (s === "verified" ? "status-approved" : s === "returned" ? "status-rejected" : s === "submitted" ? "status-pending" : "");

export function TrackerTab({ canReopen }: { canReopen: boolean }) {
  const { t } = useT();
  const [month, setMonth] = useState(currentMonth);
  const [office, setOffice] = useState("");
  const [rows, setRows] = useState<TrackerRow[] | null>(null);
  const [failed, setFailed] = useState(false);
  const [bulk, setBulk] = useState<DtrResponse[] | null>(null);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const [reopening, setReopening] = useState<number | null>(null);
  const [note, setNote] = useState("");

  const load = useCallback(() => {
    setFailed(false);
    setRows(null);
    api.dtrAdmin.tracker(month).then(setRows).catch(() => setFailed(true));
  }, [month]);
  useEffect(load, [load]);

  const offices = useMemo(() => [...new Set((rows ?? []).map((r) => r.office).filter(Boolean))].sort(), [rows]);
  const shown = (rows ?? []).filter((r) => !office || r.office === office);
  const done = shown.filter((r) => r.status === "verified").length;

  const printAll = async () => {
    setBusy(true);
    setError("");
    try {
      setBulk(await api.dtrAdmin.bulk(month, office));
      window.setTimeout(() => {
        window.print();
        setBulk(null);
      }, 500);
    } catch {
      setError(t.errGeneric);
    } finally {
      setBusy(false);
    }
  };

  const reopen = async (id: number) => {
    try {
      await api.dtrAdmin.reopen(id, note);
      setReopening(null);
      setNote("");
      load();
    } catch (e) {
      setError(e instanceof ApiError && e.message ? e.message : t.errGeneric);
    }
  };

  return (
    <>
      <section className="panel no-print" aria-labelledby="tr-h">
        <div className="panel-bar">
          <h2 id="tr-h" className="panel-h">{t.tabTracker}</h2>
          <button className="btn quiet" onClick={printAll} disabled={busy || !rows}>
            <Printer aria-hidden="true" size={20} /> {busy ? t.trackerPreparing : t.trackerBulk}
          </button>
        </div>
        <div className="month-nav">
          <button className="btn quiet icon" onClick={() => setMonth(shiftMonth(month, -1))} aria-label={t.prevMonth}>
            <ChevronLeft aria-hidden="true" size={20} />
          </button>
          <h3 className="month-label">{monthLabel(month)}</h3>
          <button className="btn quiet icon" onClick={() => setMonth(shiftMonth(month, 1))} aria-label={t.nextMonth}>
            <ChevronRight aria-hidden="true" size={20} />
          </button>
        </div>
        <label className="field">
          <span>{t.office}</span>
          <select value={office} onChange={(e) => setOffice(e.target.value)}>
            <option value="">{t.allOffices}</option>
            {offices.map((o) => <option key={o} value={o}>{o}</option>)}
          </select>
        </label>
        {failed && <p className="msg bad" role="alert">{t.loadFailed}</p>}
        {error && <p className="msg bad" role="alert">{error}</p>}
        {rows && <p className="lead">{t.trackerSummary(done, shown.length)}</p>}
        {rows && shown.length === 0 && <p className="muted">{t.trackerEmpty}</p>}
        <ul className="rows flat">
          {shown.map((r) => (
            <li key={r.employee_no} className="row">
              <span className="row-main">
                <span className="rail-slot">{r.name}</span>
                <span className="rail-state">{r.employee_no}{r.office ? ` · ${r.office}` : ""}</span>
                {canReopen && r.status === "verified" && reopening !== r.id && (
                  <button className="link-btn start" onClick={() => { setReopening(r.id); setNote(""); }}>{t.trackerReopen}</button>
                )}
                {reopening === r.id && (
                  <span className="reject-box">
                    <label className="field">
                      <span>{t.trackerReopenWhy}</span>
                      <textarea rows={2} maxLength={300} value={note} onChange={(e) => setNote(e.target.value)} />
                    </label>
                    <span className="actions">
                      <button className="btn primary" disabled={note.trim().length < 3} onClick={() => reopen(r.id)}>{t.trackerReopen}</button>
                      <button className="btn quiet" onClick={() => setReopening(null)}>{t.cancel}</button>
                    </span>
                  </span>
                )}
              </span>
              <span className="row-side"><span className={`tag ${tone(r.status)}`}>{stLabel(t, r.status)}</span></span>
            </li>
          ))}
        </ul>
      </section>
      {bulk && (
        <div className="bulk-sheet">
          {bulk.map((d) => (
            <div key={d.employee.id} className="bulk-page">
              <Form48 data={d} code={d.submission?.verify_code} />
            </div>
          ))}
        </div>
      )}
    </>
  );
}
