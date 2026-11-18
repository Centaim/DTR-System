import { useEffect, useState } from "react";
import { ChevronLeft, ChevronRight, Download } from "lucide-react";
import { api, type SummaryData } from "../../lib/api";
import { useT } from "../../lib/i18n";

const hm = (m: number) => (m === 0 ? "" : `${Math.floor(m / 60)}h ${String(m % 60).padStart(2, "0")}m`);

function shift(month: string, by: number): string {
  const [y = 0, m = 1] = month.split("-").map(Number);
  const d = new Date(Date.UTC(y, m - 1 + by, 1));
  return `${d.getUTCFullYear()}-${String(d.getUTCMonth() + 1).padStart(2, "0")}`;
}

const nowMonth = () =>
  new Intl.DateTimeFormat("en-CA", { timeZone: "Asia/Manila", year: "numeric", month: "2-digit" })
    .format(new Date())
    .slice(0, 7);

const label = (month: string) => {
  const [y = 0, m = 1] = month.split("-").map(Number);
  return new Intl.DateTimeFormat("en-PH", { timeZone: "UTC", month: "long", year: "numeric" }).format(
    new Date(Date.UTC(y, m - 1, 1)),
  );
};

export function ReportsTab() {
  const { t } = useT();
  const [month, setMonth] = useState(nowMonth);
  const [office, setOffice] = useState("");
  const [offices, setOffices] = useState<string[]>([]);
  const [data, setData] = useState<SummaryData | null>(null);
  const [failed, setFailed] = useState(false);
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    let live = true;
    setData(null);
    setFailed(false);
    api.reports
      .summary(month, office)
      .then((r) => {
        if (!live) return;
        setData(r);
        if (office === "") setOffices([...new Set(r.rows.map((x) => x.office).filter(Boolean))].sort());
      })
      .catch(() => live && setFailed(true));
    return () => {
      live = false;
    };
  }, [month, office]);

  const download = async () => {
    setBusy(true);
    try {
      const blob = await api.reports.csv(month, office);
      const url = URL.createObjectURL(blob);
      const a = document.createElement("a");
      a.href = url;
      a.download = `attendance-summary-${month}.csv`;
      a.click();
      URL.revokeObjectURL(url);
    } catch {
      setFailed(true);
    } finally {
      setBusy(false);
    }
  };

  return (
    <section className="panel" aria-labelledby="rep-h">
      <div className="panel-bar">
        <h2 id="rep-h" className="panel-h">
          {t.monthReport}
        </h2>
        <button className="btn quiet" onClick={download} disabled={busy || !data}>
          <Download aria-hidden="true" size={20} /> {busy ? t.downloading : t.downloadCsv}
        </button>
      </div>

      <div className="month-nav">
        <button className="btn quiet icon" onClick={() => setMonth(shift(month, -1))} aria-label={t.prevMonth}>
          <ChevronLeft aria-hidden="true" size={20} />
        </button>
        <h3 className="month-label">{label(month)}</h3>
        <button className="btn quiet icon" onClick={() => setMonth(shift(month, 1))} aria-label={t.nextMonth}>
          <ChevronRight aria-hidden="true" size={20} />
        </button>
      </div>

      <label className="field">
        <span>{t.office}</span>
        <select value={office} onChange={(e) => setOffice(e.target.value)}>
          <option value="">{t.allOffices}</option>
          {offices.map((o) => (
            <option key={o} value={o}>
              {o}
            </option>
          ))}
        </select>
      </label>

      {failed && (
        <p className="msg bad" role="alert">
          {t.loadFailed}
        </p>
      )}
      {!data && !failed && <p className="muted" role="status">{t.photoLoading}</p>}
      {data && (
        <>
          <p className="muted small">{t.workingDays(data.working_days)}</p>
          {data.rows.length === 0 ? (
            <p className="muted">{t.noRows}</p>
          ) : (
            <div className="paper-scroll flat-scroll">
              <table className="data wide">
                <thead>
                  <tr>
                    <th scope="col">{t.repName}</th>
                    <th scope="col">{t.repPresent}</th>
                    <th scope="col">{t.repLate}</th>
                    <th scope="col">{t.repAbsent}</th>
                    <th scope="col">{t.repExcused}</th>
                    <th scope="col">{t.repUndertime}</th>
                    <th scope="col">{t.repReview}</th>
                  </tr>
                </thead>
                <tbody>
                  {data.rows.map((r) => (
                    <tr key={r.employee_no}>
                      <th scope="row">
                        {r.name}
                        <span className="sub">
                          {r.employee_no}
                          {r.office ? ` · ${r.office}` : ""}
                        </span>
                        {r.habitual_late && <span className="tag warn">{t.habitualTag}</span>}
                      </th>
                      <td>{r.present_days}</td>
                      <td>{r.late_days}</td>
                      <td>{r.absent_days}</td>
                      <td>{r.excused_days}</td>
                      <td>{hm(r.undertime_min)}</td>
                      <td>{r.incomplete_days || ""}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
          <p className="muted small">{t.reportNote}</p>
          <p className="muted small">{t.habitualNote(data.habitual_threshold)}</p>
        </>
      )}
    </section>
  );
}
