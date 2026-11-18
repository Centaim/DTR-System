import { useCallback, useEffect, useState } from "react";
import { api, type DashboardData } from "../../lib/api";
import { slotLabel, useT } from "../../lib/i18n";
import { formatTime } from "../../lib/time";

export function DashboardTab() {
  const { t } = useT();
  const [d, setD] = useState<DashboardData | null>(null);
  const [failed, setFailed] = useState(false);

  const load = useCallback(() => {
    api.reports.dashboard().then((r) => { setD(r); setFailed(false); }).catch(() => setFailed(true));
  }, []);

  useEffect(() => {
    load();
    const id = window.setInterval(load, 30_000);
    return () => window.clearInterval(id);
  }, [load]);

  if (failed && !d) {
    return (
      <p className="msg bad" role="alert">
        {t.loadFailed} <button className="link-btn" onClick={load}>{t.retry}</button>
      </p>
    );
  }
  if (!d) return <p className="muted" role="status">{t.photoLoading}</p>;

  const attention = [
    d.pending_requests > 0 && t.attPending(d.pending_requests),
    d.open_flags > 0 && t.attFlags(d.open_flags),
    d.offline_today > 0 && t.attOffline(d.offline_today),
  ].filter(Boolean) as string[];

  return (
    <div className="admin-cols">
      <section className="panel" aria-labelledby="today-b">
        <h2 id="today-b" className="panel-h">
          {t.tabDashboard}
        </h2>
        <p className="lead" role="status">
          {d.working_day ? t.notInSentence(d.not_in, d.total) : t.nonWorkingDay(d.holiday)}
        </p>
        <div className="paper-scroll flat-scroll">
          <table className="data compact">
            <thead>
              <tr>
                <th scope="col">{t.colOffice}</th>
                <th scope="col">{t.colTotal}</th>
                <th scope="col">{t.colIn}</th>
                <th scope="col">{t.colLate}</th>
                <th scope="col">{t.colNotIn}</th>
                <th scope="col">{t.colLeave}</th>
              </tr>
            </thead>
            <tbody>
              {d.offices.map((o) => (
                <tr key={o.office}>
                  <th scope="row">{o.office}</th>
                  <td>{o.total}</td>
                  <td>{o.present}</td>
                  <td>{o.late}</td>
                  <td>{d.working_day ? o.not_in : "–"}</td>
                  <td>{o.excused}</td>
                </tr>
              ))}
            </tbody>
            <tfoot>
              <tr>
                <th scope="row">{t.colTotal}</th>
                <td>{d.total}</td>
                <td>{d.present}</td>
                <td>{d.late}</td>
                <td>{d.working_day ? d.not_in : "–"}</td>
                <td>{d.excused}</td>
              </tr>
            </tfoot>
          </table>
        </div>
      </section>

      <div className="stack">
        <section className="panel" aria-labelledby="att-h">
          <h2 id="att-h" className="panel-h">
            {t.needsAttention}
          </h2>
          {attention.length === 0 ? (
            <p className="muted">{t.attNone}</p>
          ) : (
            <ul className="plain">
              {attention.map((a) => (
                <li key={a}>{a}</li>
              ))}
            </ul>
          )}
        </section>

        <section className="panel" aria-labelledby="rec-h">
          <h2 id="rec-h" className="panel-h">
            {t.recentActivity}
          </h2>
          {d.recent.length === 0 && <p className="muted">{t.noActivity}</p>}
          <ul className="rows flat">
            {d.recent.map((r, i) => (
              <li key={i} className="row">
                <span className="row-main">
                  <span className="rail-slot">{r.employee}</span>
                  <span className="rail-state">
                    {slotLabel(t, r.slot)}
                    {r.offline ? ` · ${t.offline}` : ""}
                  </span>
                </span>
                <span className="row-side rail-time">{formatTime(r.at)}</span>
              </li>
            ))}
          </ul>
        </section>
      </div>
    </div>
  );
}
