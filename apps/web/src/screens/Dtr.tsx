import { useCallback, useEffect, useState } from "react";
import { ChevronLeft, ChevronRight, Printer, ShieldCheck } from "lucide-react";
import { ApiError, api, type DtrDay, type DtrResponse } from "../lib/api";
import { useT, type Strings } from "../lib/i18n";
import { currentMonth, monthLabel, shiftMonth, to12 } from "../lib/month";
import { Form48 } from "../components/Form48";

const stLabel = (t: Strings, s: string) => t[`dtrSt_${s}` as keyof Strings] as string;
const hm = (m: number) => `${Math.floor(m / 60)}h ${String(m % 60).padStart(2, "0")}m`;

function DayCard({ d, t }: { d: DtrDay; t: Strings }) {
  const has = d.am_in || d.am_out || d.pm_in || d.pm_out;
  const off = d.rest ?? (d.weekday === "Saturday" || d.weekday === "Sunday");
  return (
    <li className="day-card" data-muted={!has || undefined}>
      <span className="day-num">
        {d.day}
        <small>{d.weekday.slice(0, 3)}</small>
      </span>
      <span className="row-main">
        {has ? (
          <span className="rail-slot">
            {to12(d.am_in) || "—"} – {to12(d.am_out) || "—"} · {to12(d.pm_in) || "—"} – {to12(d.pm_out) || "—"}
          </span>
        ) : (
          <span className="rail-state">{d.mark || (off ? d.weekday : t.dtrNoLogs)}</span>
        )}
        {d.incomplete && <span className="rail-state flag">! {t.dtrIncomplete}</span>}
        {d.adjusted && <span className="rail-state">* {t.dtrAdjusted}</span>}
      </span>
      {d.undertime_min > 0 && <span className="rail-time">{hm(d.undertime_min)}</span>}
    </li>
  );
}

export function Dtr({ onSignOut }: { onSignOut: (sessionEnded: boolean) => void }) {
  const { t } = useT();
  const [month, setMonth] = useState(currentMonth);
  const [data, setData] = useState<DtrResponse | null>(null);
  const [failed, setFailed] = useState(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");

  const load = useCallback(() => {
    setFailed(false);
    return api
      .dtr(month)
      .then(setData)
      .catch((e) => {
        if (e instanceof ApiError && e.status === 401) onSignOut(true);
        else setFailed(true);
      });
  }, [month, onSignOut]);

  useEffect(() => {
    setData(null);
    setError("");
    void load();
  }, [load]);

  const certify = async () => {
    setBusy(true);
    setError("");
    try {
      await api.certifyDtr(month);
      await load();
    } catch (e) {
      setError(e instanceof ApiError && e.message ? e.message : t.errGeneric);
    } finally {
      setBusy(false);
    }
  };

  const sub = data?.submission ?? null;
  const live = sub && (sub.status === "submitted" || sub.status === "verified");

  return (
    <div className="shell dtr-shell">
      <header className="topbar no-print">
        <span className="brand">{t.navDtr}</span>
        <button className="btn quiet" onClick={() => window.print()} disabled={!data}>
          <Printer aria-hidden="true" size={20} /> {t.printDtr}
        </button>
      </header>

      <main className="main dtr-main with-nav">
        <div className="month-nav no-print">
          <button className="btn quiet icon" onClick={() => setMonth(shiftMonth(month, -1))} aria-label={t.prevMonth}>
            <ChevronLeft aria-hidden="true" size={20} />
          </button>
          <h1 className="month-label">{monthLabel(month)}</h1>
          <button className="btn quiet icon" onClick={() => setMonth(shiftMonth(month, 1))} aria-label={t.nextMonth}>
            <ChevronRight aria-hidden="true" size={20} />
          </button>
        </div>

        {failed && <p className="msg bad no-print" role="alert">{t.dtrFailed}</p>}
        {!data && !failed && <p className="muted no-print" role="status">{t.dtrLoading}</p>}

        {data && (
          <section className="panel form no-print" aria-live="polite">
            {sub && (
              <p className={`msg ${sub.status === "verified" ? "ok" : sub.status === "submitted" ? "warn" : "bad"}`}>
                {stLabel(t, sub.status)} · {t.dtrVersion(sub.version)}
                {sub.note ? ` · ${t.dtrReturnNote(sub.note)}` : ""}
              </p>
            )}
            {data.can_certify ? (
              <>
                <p className="muted small">{t.dtrCertifyNote}</p>
                <button className="btn primary" onClick={certify} disabled={busy}>
                  <ShieldCheck aria-hidden="true" size={20} /> {busy ? t.dtrCertifying : t.dtrCertify}
                </button>
              </>
            ) : (
              !live && <p className="muted small">{t.dtrMonthOpen}</p>
            )}
            {error && <p className="msg bad" role="alert">{error}</p>}
          </section>
        )}

        {data && (
          <>
            <ul className="day-cards no-print">
              {data.sheet.days.map((d) => (
                <DayCard key={d.day} d={d} t={t} />
              ))}
            </ul>
            <div className="paper-scroll wide-only">
              <Form48 data={data} code={live ? sub?.verify_code : undefined} />
            </div>
            <p className="muted small no-print">{t.dtrSample}</p>
          </>
        )}
      </main>
    </div>
  );
}
