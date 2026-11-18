import { useEffect, useState } from "react";
import QRCode from "qrcode";
import type { DtrDay, DtrResponse } from "../lib/api";
import { useT } from "../lib/i18n";
import { monthLabel, to12 } from "../lib/month";

const hasLogs = (d: DtrDay) => Boolean(d.am_in || d.am_out || d.pm_in || d.pm_out);

/** CSC Form No. 48 as printed. The form's own wording stays in English. */
export function Form48({ data, code }: { data: Pick<DtrResponse, "employee" | "sheet" | "official_hours">; code?: string }) {
  const { t } = useT();
  const [qr, setQr] = useState("");
  const month = data.sheet.month;
  const days = data.sheet.days;
  const total = data.sheet.total_undertime_min;
  const regular = data.official_hours
    .map((h) => `${to12(h.am_start)}–${to12(h.am_end)}, ${to12(h.pm_start)}–${to12(h.pm_end)}`)
    .join("  /  ");

  useEffect(() => {
    if (!code) return setQr("");
    let live = true;
    void QRCode.toDataURL(`${window.location.origin}/verify/${code}`, { margin: 0, width: 200 }).then((u) => live && setQr(u));
    return () => {
      live = false;
    };
  }, [code]);

  return (
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
                    <td>{d.undertime_min > 0 ? Math.floor(d.undertime_min / 60) : ""}</td>
                    <td>{d.undertime_min > 0 ? d.undertime_min % 60 : ""}</td>
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
        I CERTIFY on my honor that the above is a true and correct report of the hours of work performed, record of
        which was made daily at the time of arrival and departure from office.
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
      {qr && (
        <div className="f48-qr">
          <img src={qr} alt="" />
          <span>{t.dtrVerifyCaption}</span>
        </div>
      )}
    </article>
  );
}
