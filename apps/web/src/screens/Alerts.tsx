import { useCallback, useEffect, useState, type FormEvent } from "react";
import { ApiError, api, type Announcement, type Employee, type NotificationItem } from "../lib/api";
import { useT, type Strings } from "../lib/i18n";
import { formatTime } from "../lib/time";

const when = (iso: string) =>
  new Intl.DateTimeFormat("en-PH", { timeZone: "Asia/Manila", month: "short", day: "numeric" }).format(new Date(iso)) + ", " + formatTime(iso);
const label = (t: Strings, k: string) => (t[`n_${k}` as keyof Strings] as string) ?? k;

export function Alerts({ employee, onRead }: { employee: Employee; onRead: () => void }) {
  const { t } = useT();
  const [items, setItems] = useState<NotificationItem[] | null>(null);
  const [anns, setAnns] = useState<Announcement[]>([]);
  const [failed, setFailed] = useState(false);
  const [f, setF] = useState({ title: "", body: "", office: "" });
  const [error, setError] = useState("");
  const hr = employee.role === "hr_admin" || employee.role === "super_admin";

  const load = useCallback(() => {
    setFailed(false);
    Promise.all([api.notifications.list(), api.announcements.list()])
      .then(([n, a]) => {
        setItems(n.items);
        setAnns(a);
        if (n.unread > 0) void api.notifications.markRead().then(onRead);
      })
      .catch(() => setFailed(true));
  }, [onRead]);
  useEffect(load, [load]);

  const post = async (ev: FormEvent) => {
    ev.preventDefault();
    setError("");
    try {
      await api.announcements.create(f);
      setF({ title: "", body: "", office: "" });
      load();
    } catch (e) {
      setError(e instanceof ApiError && e.message ? e.message : t.errGeneric);
    }
  };

  return (
    <div className="shell">
      <header className="topbar"><span className="brand">{t.alertsTitle}</span></header>
      <main className="main admin-main with-nav">
        {failed && <p className="msg bad" role="alert">{t.loadFailed} <button className="link-btn" onClick={load}>{t.retry}</button></p>}
        <div className="admin-cols">
          <section className="panel" aria-labelledby="an-h">
            <h1 id="an-h" className="panel-h">{t.alertsAnnouncements}</h1>
            {anns.length === 0 && <p className="muted">{t.annEmpty}</p>}
            <ul className="rows flat">
              {anns.map((a) => (
                <li key={a.id} className="row approval">
                  <span className="row-main">
                    <span className="rail-slot">{a.title}</span>
                    <span className="rail-state">{when(a.created_at)}</span>
                    <span className="pre">{a.body}</span>
                    {hr && <span className="rail-state">{t.annStats(a.ack_count, a.audience)}</span>}
                    {a.acked ? (
                      <span className="tag status-approved fit">{t.annAcked}</span>
                    ) : (
                      <span className="actions">
                        <button className="btn quiet" onClick={() => api.announcements.ack(a.id).finally(load)}>{t.annAck}</button>
                      </span>
                    )}
                  </span>
                </li>
              ))}
            </ul>
          </section>

          <div className="stack">
            <section className="panel" aria-labelledby="no-h">
              <h2 id="no-h" className="panel-h">{t.alertsNotifications}</h2>
              {items?.length === 0 && <p className="muted">{t.alertsEmpty}</p>}
              <ul className="rows flat">
                {items?.map((n) => (
                  <li key={n.id} className="row">
                    <span className="row-main">
                      <span className="rail-slot">{label(t, n.kind)}</span>
                      <span className="rail-state">{[n.detail, when(n.created_at)].filter(Boolean).join(" · ")}</span>
                    </span>
                  </li>
                ))}
              </ul>
            </section>

            {hr && (
              <form className="panel form" onSubmit={post} noValidate>
                <h2 className="panel-h">{t.annCompose}</h2>
                <label className="field"><span>{t.annTitle}</span><input value={f.title} maxLength={120} onChange={(e) => setF({ ...f, title: e.target.value })} /></label>
                <label className="field"><span>{t.annBody}</span><textarea rows={4} maxLength={4000} value={f.body} onChange={(e) => setF({ ...f, body: e.target.value })} /></label>
                <label className="field"><span>{t.annOffice}</span><input value={f.office} maxLength={80} onChange={(e) => setF({ ...f, office: e.target.value })} /></label>
                <div aria-live="polite">{error && <p className="msg bad">{error}</p>}</div>
                <button className="btn primary" type="submit" disabled={f.title.trim().length < 3 || f.body.trim().length < 3}>{t.annPost}</button>
              </form>
            )}
          </div>
        </div>
      </main>
    </div>
  );
}
