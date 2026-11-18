import { useEffect, useMemo, useState } from "react";
import QRCode from "qrcode";
import { ChevronLeft, Printer } from "lucide-react";
import { api, type AdminEmployee } from "../../lib/api";
import { useT } from "../../lib/i18n";

export function IdCards({ onBack }: { onBack: () => void }) {
  const { t } = useT();
  const [all, setAll] = useState<AdminEmployee[] | null>(null);
  const [failed, setFailed] = useState(false);
  const [q, setQ] = useState("");
  const [office, setOffice] = useState("");
  const [picked, setPicked] = useState<Set<number>>(new Set());
  const [qr, setQr] = useState<Record<string, string>>({});

  useEffect(() => {
    api.admin.employees("").then((l) => setAll(l.filter((e) => e.active))).catch(() => setFailed(true));
  }, []);

  const offices = useMemo(() => [...new Set((all ?? []).map((e) => e.office).filter(Boolean))].sort(), [all]);
  const shown = useMemo(
    () =>
      (all ?? []).filter(
        (e) =>
          (!office || e.office === office) &&
          (!q || `${e.full_name} ${e.employee_no}`.toLowerCase().includes(q.toLowerCase())),
      ),
    [all, q, office],
  );
  const chosen = useMemo(() => (all ?? []).filter((e) => picked.has(e.id)), [all, picked]);

  useEffect(() => {
    let live = true;
    const missing = chosen.filter((e) => !qr[e.employee_no]);
    if (missing.length === 0) return;
    void Promise.all(
      missing.map(async (e) => [
        e.employee_no,
        await QRCode.toDataURL(`DTR:${e.employee_no}`, { margin: 0, width: 320, errorCorrectionLevel: "M" }),
      ] as const),
    ).then((pairs) => live && setQr((p) => ({ ...p, ...Object.fromEntries(pairs) })));
    return () => {
      live = false;
    };
  }, [chosen, qr]);

  const toggle = (id: number) =>
    setPicked((p) => {
      const n = new Set(p);
      if (n.has(id)) n.delete(id);
      else n.add(id);
      return n;
    });

  return (
    <div className="admin-cols cards-cols">
      <section className="panel form no-print" aria-labelledby="cards-h">
        <button className="link-btn back" onClick={onBack}>
          <ChevronLeft aria-hidden="true" size={18} /> {t.backToList}
        </button>
        <h2 id="cards-h" className="title">
          {t.cardsTitle}
        </h2>
        <p className="muted small">{t.cardsHelp}</p>

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
        <label className="field">
          <span className="sr-only">{t.search}</span>
          <input type="search" placeholder={t.search} value={q} onChange={(e) => setQ(e.target.value)} />
        </label>
        <div className="actions">
          <button className="btn quiet" onClick={() => setPicked(new Set([...picked, ...shown.map((e) => e.id)]))}>
            {t.cardsSelectShown}
          </button>
          <button className="btn quiet" onClick={() => setPicked(new Set())} disabled={picked.size === 0}>
            {t.cardsClear}
          </button>
        </div>

        {failed && <p className="msg bad" role="alert">{t.loadFailed}</p>}
        <ul className="rows flat pick-list">
          {shown.map((e) => (
            <li key={e.id} className="row">
              <label className="pick">
                <input type="checkbox" checked={picked.has(e.id)} onChange={() => toggle(e.id)} />
                <span className="row-main">
                  <span className="rail-slot">{e.full_name}</span>
                  <span className="rail-state">
                    {e.employee_no}
                    {e.office ? ` · ${e.office}` : ""}
                  </span>
                </span>
              </label>
            </li>
          ))}
        </ul>

        <button className="btn primary" onClick={() => window.print()} disabled={chosen.length === 0}>
          <Printer aria-hidden="true" size={20} /> {t.cardsPrint(chosen.length)}
        </button>
      </section>

      <section className="id-wrap" aria-label={t.cardsTitle}>
        {chosen.length === 0 ? (
          <p className="muted no-print">{t.cardsNone}</p>
        ) : (
          <div className="id-sheet">
            {chosen.map((e) => (
              <article key={e.id} className="id-card">
                <div className="id-text">
                  <p className="id-head">
                    {t.appName} · {t.cardHeader}
                  </p>
                  <p className="id-name">{e.full_name}</p>
                  <p className="id-line">{e.position || " "}</p>
                  <p className="id-line">{e.office || " "}</p>
                  <p className="id-no">{e.employee_no}</p>
                </div>
                {qr[e.employee_no] ? (
                  <img className="id-qr" src={qr[e.employee_no]} alt={`QR ${e.employee_no}`} />
                ) : (
                  <span className="id-qr" />
                )}
              </article>
            ))}
          </div>
        )}
      </section>
    </div>
  );
}
