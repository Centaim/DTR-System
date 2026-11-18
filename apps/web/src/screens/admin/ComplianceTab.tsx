import { useCallback, useEffect, useState, type FormEvent } from "react";
import { ChevronLeft, ChevronRight, Download } from "lucide-react";
import { ApiError, NetworkError, api, type AccessRow } from "../../lib/api";
import { saveBlob } from "../../lib/download";
import { useT, type Strings } from "../../lib/i18n";
import { currentMonth, monthLabel, shiftMonth } from "../../lib/month";

const roleName = (t: Strings, r: string) => (t[`role_${r}` as keyof Strings] as string) ?? r;
const attention = (t: Strings, a: string) => t[`att_${a}` as keyof Strings] as string;

export function ComplianceTab({ canExportPeople }: { canExportPeople: boolean }) {
  const { t } = useT();
  const [month, setMonth] = useState(() => shiftMonth(currentMonth(), -1));
  const [office, setOffice] = useState("");
  const [photos, setPhotos] = useState(false);
  const [busy, setBusy] = useState("");
  const [error, setError] = useState("");
  const [rows, setRows] = useState<AccessRow[] | null>(null);
  const [failed, setFailed] = useState(false);
  const [employeeNo, setEmployeeNo] = useState("");

  const load = useCallback(() => {
    setFailed(false);
    api.compliance.accessReview().then(setRows).catch(() => setFailed(true));
  }, []);
  useEffect(load, [load]);

  const run = async (name: string, work: () => Promise<void>) => {
    setBusy(name);
    setError("");
    try {
      await work();
    } catch (e) {
      setError(e instanceof NetworkError ? t.errNetwork : e instanceof ApiError && e.message ? e.message : t.errGeneric);
    } finally {
      setBusy("");
    }
  };

  const flagged = (rows ?? []).filter((r) => r.attention);

  return (
    <div className="admin-cols">
      <div className="stack">
        <section className="panel form" aria-labelledby="pk-h">
          <h2 id="pk-h" className="panel-h">{t.packTitle}</h2>
          <p className="muted small">{t.packNote}</p>
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
            <span>{t.packOffice}</span>
            <input value={office} maxLength={80} onChange={(e) => setOffice(e.target.value)} />
          </label>
          <label className="pick">
            <input type="checkbox" checked={photos} onChange={(e) => setPhotos(e.target.checked)} />
            <span>{t.packPhotos}</span>
          </label>
          <button
            className="btn primary"
            disabled={busy !== ""}
            onClick={() => run("pack", async () => saveBlob(await api.compliance.auditPack(month, office.trim(), photos), `audit-pack-${month}.zip`))}
          >
            <Download aria-hidden="true" size={20} /> {busy === "pack" ? t.packPreparing : t.packDownload}
          </button>
        </section>

        {canExportPeople && (
          <form
            className="panel form"
            noValidate
            onSubmit={(ev: FormEvent) => {
              ev.preventDefault();
              const no = employeeNo.trim().toUpperCase();
              void run("data", async () => saveBlob(await api.compliance.employeeData(no), `dtr-data-${no}.zip`));
            }}
          >
            <h2 className="panel-h">{t.dsTitle}</h2>
            <p className="muted small">{t.dsNote}</p>
            <label className="field">
              <span>{t.dsEmployee}</span>
              <input value={employeeNo} onChange={(e) => setEmployeeNo(e.target.value)} autoCapitalize="characters" spellCheck={false} />
            </label>
            <button className="btn quiet" type="submit" disabled={busy !== "" || !employeeNo.trim()}>
              <Download aria-hidden="true" size={20} /> {busy === "data" ? t.packPreparing : t.dsDownload}
            </button>
          </form>
        )}
        <div aria-live="polite">{error && <p className="msg bad">{error}</p>}</div>
      </div>

      <section className="panel" aria-labelledby="ac-h">
        <div className="panel-bar">
          <h2 id="ac-h" className="panel-h">{t.accessTitle}</h2>
          <button
            className="btn quiet"
            disabled={busy !== "" || !rows}
            onClick={() => run("csv", async () => saveBlob(await api.compliance.accessReviewCsv(), "access-review.csv"))}
          >
            <Download aria-hidden="true" size={20} /> {t.accessCsv}
          </button>
        </div>
        <p className="muted small">{t.accessNote}</p>
        {failed && (
          <p className="msg bad" role="alert">
            {t.loadFailed} <button className="link-btn" onClick={load}>{t.retry}</button>
          </p>
        )}
        {rows && <p className={`msg ${flagged.length ? "warn" : "ok"}`}>{flagged.length ? t.accessAttention(flagged.length) : t.accessAllClear}</p>}
        <ul className="rows flat">
          {rows?.map((r) => (
            <li key={r.employee_no} className="row">
              <span className="row-main">
                <span className="rail-slot">{r.name} {!r.active && <span className="tag bad">{t.inactive}</span>}</span>
                <span className="rail-state">{r.employee_no} · {r.last_login ? t.accessLast(r.last_login.slice(0, 16)) : t.accessNever}</span>
                {r.attention && <span className="tag warn fit">{attention(t, r.attention)}</span>}
              </span>
              <span className="row-side">
                <span className="tag">{roleName(t, r.role)}</span>
                {r.two_step && <span className="rail-state">{t.accessTwoStep}</span>}
              </span>
            </li>
          ))}
        </ul>
      </section>
    </div>
  );
}
