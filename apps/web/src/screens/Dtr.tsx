import { useEffect, useState } from "react";
import { ChevronLeft, ChevronRight, Printer } from "lucide-react";
import { ApiError, api, type DtrDay, type DtrResponse } from "../lib/api";
import { useT } from "../lib/i18n";

const to12 = (hhmm: string) => {
  if (!hhmm) return "";
  const [h = "0", m = "00"] = hhmm.split(":");
  return `${Number(h) % 12 || 12}:${m}`;
};

const hasLogs = (d: DtrDay) => Boolean(d.am_in || d.am_out || d.pm_in || d.pm_out);

function shiftMonth(month: string, by: number): string {
  const [y = 0, m = 1] = month.split("-").map(Number);
  const d = new Date(Date.UTC(y, m - 1 + by, 1));
  return `${d.getUTCFullYear()}-${String(d.getUTCMonth() + 1).padStart(2, "0")}`;
}

function currentMonth(): string {
  const parts = new Intl.DateTimeFormat("en-CA", { timeZone: "Asia/Manila", year: "numeric", month: "2-digit" })
    .formatToParts(new Date());
  const get = (t: string) => parts.find((p) => p.type === t)?.value ?? "";
  return `${get("year")}-${get("month")}`;
}

function monthLabel(month: string): string {
  const [y = 0, m = 1] = month.split("-").map(Number);
  return new Intl.DateTimeFormat("en-PH", { timeZone: "UTC", month: "long", year: "numeric" }).format(
    new Date(Date.UTC(y, m - 1, 1)),
  );
}

export function Dtr({ onSignOut }: { onSignOut: (sessionEnded: boolean) => void }) {
  const { t } = useT();
  const [month, setMonth] = useState(currentMonth);
  const [data, setData] = useState<DtrResponse | null>(null);
  const [failed, setFailed] = useState(false);

  useEffect(() => {
    let live = true;
    setData(null);
    setFailed(false);
    api
      .dtr(month)
      .then((r) => live && setData(r))
      .catch((e) => {
        if (!live) return;
        if (e instanceof ApiError && e.status === 401) onSignOut(true);
        else setFailed(true);
      });
    return () => {
      live = false;
    };
  }, [month, onSignOut]);

  const regular = (data?.official_hours ?? [])
    .map((h) => `${to12(h.am_start)}–${to12(h.am_end)}, ${to12(h.pm_start)}–${to12(h.pm_end)}`)
    .join("  /  ");
  const anyIncomplete = data?.sheet.days.some((d) => d.incomplete) ?? false;
  const anyAdjusted = data?.sheet.days.some((d) => d.adjusted) ?? false;
  const total = data?.sheet.total_undertime_min ?? 0;
  const days = data?.sheet.days ?? [];

  return (
    <div className="shell dtr-shell">
      <header className="topbar no-print">
        <span className="brand">{t.navDtr}</span>
        <button className="btn primary" onClick={() => window.print()} disabled={!data}>
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

        {failed && (
          <p className="msg bad no-print" role="alert">
            {t.dtrFailed}
          </p>
        )}
        {!data && !failed && <p className="muted no-print" role="status">{t.dtrLoading}</p>}

        {data && (
          <div className="paper-scroll">
            <article className="form48" aria-label={`Daily Time Record, ${monthLabel(month)}`}>
              <p className="f48-ref">Civil Service Form No. 48</p>
              <h2 className="f48-title">DAILY TIME RECORD</h2>
              <p className="f48-rule">-----o0o-----</p>

              <p className="f48-name">{data.employee.full_name}</p>
              <p className="f48-cap">(Name)</p>

              <div className="f48-meta">
                <p>
                  For the month of <span className="f48-fill">{monthLabel(month)}</span>
                </p>
                <p>
                  Official hours for arrival and departure
                  <span className="f48-fill block">Regular days: {regular}</span>
                  <span className="f48-fill block">Saturdays:</span>
                </p>
              </div>

              <table className="f48-table">
                <thead>
                  <tr>
                    <th rowSpan={2}>Day</th>
                    <th colSpan={2}>A.M.</th>
                    <th colSpan={2}>P.M.</th>
                    <th colSpan={2}>Undertime</th>
                  </tr>
                  <tr>
                    <th>Arrival</th>
                    <th>Departure</th>
                    <th>Arrival</th>
                    <th>Departure</th>
                    <th>Hours</th>
                    <th>Minutes</th>
                  </tr>
                </thead>
                <tbody>
                  {Array.from({ length: 31 }, (_, i) => days[i]).map((d, i) => {
                    if (!d) {
                      return (
                        <tr key={i}>
                          <td>{i + 1}</td>
                          <td colSpan={6} />
                        </tr>
                      );
                    }
                    const weekend = d.weekday === "Saturday" || d.weekday === "Sunday";
                    return (
                      <tr key={d.day} data-incomplete={d.incomplete || undefined}>
                        <td>
                          {d.day}
                          {d.incomplete && <span className="no-print flag" title={t.dtrIncomplete}> !</span>}
                          {d.adjusted && <span className="no-print flag" title={t.dtrAdjusted}> *</span>}
                        </td>
                        {(weekend || d.mark) && !hasLogs(d) ? (
                          <td colSpan={6} className="f48-off">
                            {d.mark || d.weekday.toUpperCase()}
                          </td>
                        ) : (
                          <>
                            <td>{to12(d.am_in)}</td>
                            <td>{to12(d.am_out)}</td>
                            <td>{to12(d.pm_in)}</td>
                            <td>{to12(d.pm_out)}</td>
                            <td>{hasLogs(d) && d.undertime_min > 0 ? Math.floor(d.undertime_min / 60) : ""}</td>
                            <td>{hasLogs(d) && d.undertime_min > 0 ? d.undertime_min % 60 : ""}</td>
                          </>
                        )}
                      </tr>
                    );
                  })}
                  <tr className="f48-total">
                    <td colSpan={5}>TOTAL</td>
                    <td>{total > 0 ? Math.floor(total / 60) : ""}</td>
                    <td>{total > 0 ? total % 60 : ""}</td>
                  </tr>
                </tbody>
              </table>

              <p className="f48-cert">
                I CERTIFY on my honor that the above is a true and correct report of the hours of work performed,
                record of which was made daily at the time of arrival and departure from office.
              </p>
              <div className="f48-sign">
                <span />
                <p className="f48-cap">{data.employee.full_name}</p>
              </div>
              <p className="f48-verify">Verified as to the prescribed office hours.</p>
              <div className="f48-sign">
                <span />
                <p className="f48-cap">In Charge</p>
              </div>
            </article>
          </div>
        )}

        {data && anyIncomplete && <p className="muted small no-print">{t.dtrIncomplete}</p>}
        {data && anyAdjusted && <p className="muted small no-print">{t.dtrAdjusted}</p>}
        <p className="muted small no-print">{t.dtrSample}</p>
      </main>
    </div>
  );
}
