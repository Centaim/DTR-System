import { useRef, useState } from "react";
import { ChevronLeft, Download, FileUp } from "lucide-react";
import { ApiError, NetworkError, api, type ImportResult } from "../../lib/api";
import { useT, type Strings } from "../../lib/i18n";
import { downloadCsv } from "../../lib/csv";

const stLabel = (t: Strings, s: string) => (t[`st_${s}` as keyof Strings] as string) ?? s;

export function ImportScreen({ onBack }: { onBack: () => void }) {
  const { t } = useT();
  const input = useRef<HTMLInputElement>(null);
  const [file, setFile] = useState<File | null>(null);
  const [result, setResult] = useState<ImportResult | null>(null);
  const [done, setDone] = useState(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");

  const message = (e: unknown) =>
    e instanceof NetworkError ? t.errNetwork : e instanceof ApiError && e.message ? e.message : t.errGeneric;

  const preview = async (f: File) => {
    setFile(f);
    setResult(null);
    setError("");
    setBusy(true);
    try {
      setResult(await api.admin.importEmployees(f, false));
    } catch (e) {
      setError(message(e));
    } finally {
      setBusy(false);
    }
  };

  const commit = async () => {
    if (!file) return;
    setBusy(true);
    setError("");
    try {
      setResult(await api.admin.importEmployees(file, true));
      setDone(true);
    } catch (e) {
      setError(message(e));
    } finally {
      setBusy(false);
    }
  };

  const template = () =>
    downloadCsv("employee-import-template.csv", [
      ["Employee No", "Full Name", "Position", "Office", "Role", "Employment"],
      ["2026-0001", "Dela Cruz, Juan", "Administrative Aide I", "Records Section", "employee", "plantilla"],
    ]);

  const pins = () =>
    downloadCsv("temporary-pins.csv", [
      ["Employee No", "Full Name", "Temporary PIN"],
      ...(result?.created ?? []).map((c) => [c.employee_no, c.full_name, c.temp_pin]),
    ]);

  const s = result?.summary;

  return (
    <section className="panel form" aria-labelledby="imp-h">
      <button className="link-btn back" onClick={onBack}>
        <ChevronLeft aria-hidden="true" size={18} /> {t.backToList}
      </button>
      <h2 id="imp-h" className="title">
        {done ? t.importDoneTitle(result?.created.length ?? 0) : t.importTitle}
      </h2>

      {done && result ? (
        <>
          <p className="msg warn">{t.importPinNote}</p>
          <button className="btn primary" onClick={pins}>
            <Download aria-hidden="true" size={20} /> {t.importDownloadPins}
          </button>
          <div className="paper-scroll flat-scroll">
            <table className="data">
              <thead>
                <tr>
                  <th scope="col">{t.colEmpNo}</th>
                  <th scope="col">{t.colName}</th>
                  <th scope="col">{t.colPin}</th>
                </tr>
              </thead>
              <tbody>
                {result.created.map((c) => (
                  <tr key={c.employee_no}>
                    <th scope="row">{c.employee_no}</th>
                    <td className="left">{c.full_name}</td>
                    <td>{c.temp_pin}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
          <button className="btn quiet" onClick={onBack}>
            {t.done}
          </button>
        </>
      ) : (
        <>
          <p className="muted">{t.importHelp}</p>
          <p className="muted small">{t.importColumns}</p>
          <div className="actions">
            <input
              ref={input}
              type="file"
              className="sr-only"
              accept=".xlsx,.csv,text/csv,application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
              onChange={(e) => {
                const f = e.target.files?.[0];
                if (f) void preview(f);
                e.target.value = "";
              }}
              aria-label={t.importChoose}
            />
            <button className="btn primary" onClick={() => input.current?.click()} disabled={busy}>
              <FileUp aria-hidden="true" size={20} /> {file ? t.importAnother : t.importChoose}
            </button>
            <button className="btn quiet" onClick={template}>
              <Download aria-hidden="true" size={20} /> {t.importTemplate}
            </button>
          </div>

          <div aria-live="polite">
            {busy && !result && <p className="muted">{t.importPreviewing}</p>}
            {error && <p className="msg bad">{error}</p>}
          </div>

          {result && s && (
            <>
              <p className="lead">{t.importSummary(s.ok, s.duplicate, s.invalid)}</p>
              <div className="paper-scroll flat-scroll">
                <table className="data">
                  <thead>
                    <tr>
                      <th scope="col">{t.colStatus}</th>
                      <th scope="col" className="left">
                        {t.colName}
                      </th>
                    </tr>
                  </thead>
                  <tbody>
                    {result.rows.map((r) => (
                      <tr key={r.line}>
                        <td className="left shrink">
                          <span className={`tag ${r.status === "ok" ? "status-approved" : r.status === "invalid" ? "status-rejected" : "warn"}`}>
                            {stLabel(t, r.status)}
                          </span>
                        </td>
                        <th scope="row" className="wrap">
                          {r.full_name || "—"}
                          <span className="sub">
                            {t.colLine} {r.line} · {r.employee_no || "—"}
                          </span>
                          {r.message && <span className="sub">{r.message}</span>}
                        </th>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
              <button className="btn primary" onClick={commit} disabled={busy || s.ok === 0}>
                {busy ? t.importing : t.importConfirm(s.ok)}
              </button>
            </>
          )}
        </>
      )}
    </section>
  );
}
