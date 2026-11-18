import { useCallback, useEffect, useState } from "react";
import { ChevronLeft, Check, Flag } from "lucide-react";
import { ApiError, NetworkError, api, type AuditItem } from "../../lib/api";
import { slotLabel, useT, type Strings } from "../../lib/i18n";
import { formatTime } from "../../lib/time";
import { AuthImg } from "../../components/AuthImg";

type View = "flagged" | "sample" | "followup" | "date";
const VIEWS: View[] = ["flagged", "sample", "followup", "date"];
const viewLabel = (t: Strings, v: View) =>
  ({ flagged: t.viewFlagged, sample: t.viewSample, followup: t.viewFollowup, date: t.viewDate })[v];
const flagLabel = (t: Strings, k: string) => (t[`flag_${k}` as keyof Strings] as string) ?? k;
const outcomeLabel = (t: Strings, o: string) => (t[`outcome_${o}` as keyof Strings] as string) ?? o;
const today = () => new Intl.DateTimeFormat("en-CA", { timeZone: "Asia/Manila" }).format(new Date());
const dayTime = (iso: string) =>
  new Intl.DateTimeFormat("en-PH", { timeZone: "Asia/Manila", month: "short", day: "numeric" }).format(new Date(iso)) +
  ", " + formatTime(iso);

export function AuditTab({ canReview }: { canReview: boolean }) {
  const { t } = useT();
  const [view, setView] = useState<View>("flagged");
  const [date, setDate] = useState(today);
  const [list, setList] = useState<AuditItem[] | null>(null);
  const [failed, setFailed] = useState(false);
  const [open, setOpen] = useState<AuditItem | null>(null);

  const load = useCallback(() => {
    setFailed(false);
    setList(null);
    api.audit.list(view, date).then(setList).catch(() => setFailed(true));
  }, [view, date]);
  useEffect(load, [load]);

  if (open) {
    return (
      <Detail
        item={open}
        canReview={canReview}
        onBack={() => {
          setOpen(null);
          load();
        }}
      />
    );
  }

  return (
    <section className="panel" aria-labelledby="audit-h">
      <h2 id="audit-h" className="panel-h">
        {t.tabAudit}
      </h2>
      <p className="muted small">{t.auditNote}</p>

      <div className="seg wide" role="group" aria-label={t.tabAudit}>
        {VIEWS.map((v) => (
          <button key={v} className="seg-btn" aria-pressed={view === v} onClick={() => setView(v)}>
            {viewLabel(t, v)}
          </button>
        ))}
      </div>
      {view === "date" && (
        <label className="field">
          <span>{t.auditDate}</span>
          <input type="date" value={date} onChange={(e) => setDate(e.target.value)} />
        </label>
      )}

      {failed && (
        <p className="msg bad" role="alert">
          {t.loadFailed} <button className="link-btn" onClick={load}>{t.retry}</button>
        </p>
      )}
      {list && list.length === 0 && <p className="muted">{t.auditEmpty}</p>}
      <ul className="rows">
        {list?.map((it) => (
          <li key={it.log_id}>
            <button className="row row-btn" onClick={() => setOpen(it)}>
              <span className="row-main">
                <span className="rail-slot">{it.employee}</span>
                <span className="rail-state">
                  {slotLabel(t, it.slot)} · {dayTime(it.recorded_at)}
                </span>
                {it.flags.length > 0 && (
                  <span className="tags">
                    {it.flags.map((f) => (
                      <span key={f.kind} className="tag warn">
                        {flagLabel(t, f.kind)}
                      </span>
                    ))}
                  </span>
                )}
              </span>
              {it.review && (
                <span className="row-side">
                  <span className={`tag ${it.review.outcome === "cleared" ? "status-approved" : "status-rejected-plain"}`}>
                    {outcomeLabel(t, it.review.outcome)}
                  </span>
                </span>
              )}
            </button>
          </li>
        ))}
      </ul>
    </section>
  );
}

function Detail({ item, canReview, onBack }: { item: AuditItem; canReview: boolean; onBack: () => void }) {
  const { t } = useT();
  const [review, setReview] = useState(item.review);
  const [followUp, setFollowUp] = useState(false);
  const [note, setNote] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const [saved, setSaved] = useState(false);

  const save = async (outcome: "cleared" | "follow_up") => {
    setBusy(true);
    setError("");
    setSaved(false);
    try {
      await api.audit.review(item.log_id, outcome, outcome === "follow_up" ? note : "");
      setReview({ outcome, note: outcome === "follow_up" ? note : "", reviewer: "", reviewed_at: new Date().toISOString() });
      setFollowUp(false);
      setSaved(true);
    } catch (e) {
      setError(
        e instanceof NetworkError ? t.errNetwork : e instanceof ApiError && e.message ? e.message : t.errGeneric,
      );
    } finally {
      setBusy(false);
    }
  };

  return (
    <section className="panel form" aria-labelledby="det-h">
      <button className="link-btn back" onClick={onBack}>
        <ChevronLeft aria-hidden="true" size={18} /> {t.backToAudit}
      </button>
      <h2 id="det-h" className="title">
        {item.employee}
      </h2>
      <p className="muted">
        {item.employee_no} · {slotLabel(t, item.slot)} · {dayTime(item.recorded_at)}
        <br />
        {t.logInfo(item.channel, item.offline, item.device_id)}
      </p>

      <div className="photo-pair">
        <figure>
          <figcaption>{t.thisPhoto}</figcaption>
          <AuthImg logId={item.log_id} alt={t.thisPhoto} />
        </figure>
        <figure>
          <figcaption>{t.previousPhoto}</figcaption>
          {item.previous_log_id ? (
            <AuthImg logId={item.previous_log_id} alt={t.previousPhoto} />
          ) : (
            <p className="photo-note muted">{t.noPrevious}</p>
          )}
        </figure>
      </div>

      <h3 className="panel-h">{t.flagsHeading}</h3>
      {item.flags.length === 0 ? (
        <p className="muted">{t.noFlags}</p>
      ) : (
        <ul className="rows flat">
          {item.flags.map((f) => (
            <li key={f.kind} className="row">
              <span className="row-main">
                <span className="rail-slot">{flagLabel(t, f.kind)}</span>
                <span className="rail-state">{f.detail}</span>
              </span>
            </li>
          ))}
        </ul>
      )}

      <h3 className="panel-h">{t.reviewHeading}</h3>
      {review && (
        <p className={`msg ${review.outcome === "cleared" ? "ok" : "warn"}`} role="status">
          {outcomeLabel(t, review.outcome)}
          {review.reviewer ? ` · ${review.reviewer}` : ""}
          {review.note ? ` · ${review.note}` : ""}
        </p>
      )}
      {!canReview ? (
        <p className="muted small">{t.viewOnly}</p>
      ) : followUp ? (
        <div className="reject-box">
          <label className="field">
            <span>{t.followUpNote}</span>
            <textarea value={note} onChange={(e) => setNote(e.target.value)} rows={2} maxLength={300} />
          </label>
          <span className="actions">
            <button className="btn primary" disabled={busy || note.trim().length < 3} onClick={() => save("follow_up")}>
              {t.saveFollowUp}
            </button>
            <button className="btn quiet" onClick={() => setFollowUp(false)}>
              {t.cancel}
            </button>
          </span>
        </div>
      ) : (
        <span className="actions">
          <button className="btn primary" disabled={busy} onClick={() => save("cleared")}>
            <Check aria-hidden="true" size={20} /> {t.looksFine}
          </button>
          <button className="btn quiet" disabled={busy} onClick={() => setFollowUp(true)}>
            <Flag aria-hidden="true" size={20} /> {t.needsFollowUp}
          </button>
        </span>
      )}
      <div aria-live="polite">
        {error && <p className="msg bad">{error}</p>}
        {saved && <p className="msg ok">{t.reviewSaved}</p>}
      </div>
    </section>
  );
}
