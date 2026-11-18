import { useCallback, useEffect, useState } from "react";
import { api, type ActivityRow } from "../../lib/api";
import { useT, type Strings } from "../../lib/i18n";
import { formatTime } from "../../lib/time";

const label = (t: Strings, action: string) =>
  (t[`act_${action.replaceAll(".", "_")}` as keyof Strings] as string) ?? action;

const when = (iso: string) =>
  new Intl.DateTimeFormat("en-PH", { timeZone: "Asia/Manila", month: "short", day: "numeric" }).format(new Date(iso)) +
  ", " + formatTime(iso);

export function ActivityTab() {
  const { t } = useT();
  const [list, setList] = useState<ActivityRow[] | null>(null);
  const [failed, setFailed] = useState(false);

  const load = useCallback(() => {
    setFailed(false);
    api.audit.activity().then(setList).catch(() => setFailed(true));
  }, []);
  useEffect(load, [load]);

  return (
    <section className="panel" aria-labelledby="act-h">
      <h2 id="act-h" className="panel-h">
        {t.activityTitle}
      </h2>
      <p className="muted small">{t.activityNote}</p>
      {failed && (
        <p className="msg bad" role="alert">
          {t.loadFailed} <button className="link-btn" onClick={load}>{t.retry}</button>
        </p>
      )}
      {list && list.length === 0 && <p className="muted">{t.activityEmpty}</p>}
      <ul className="rows flat">
        {list?.map((r) => (
          <li key={r.id} className="row">
            <span className="row-main">
              <span className="rail-slot">{label(t, r.action)}</span>
              <span className="rail-state">
                {r.actor || "—"} · {r.target}
              </span>
            </span>
            <span className="row-side rail-time">{when(r.at)}</span>
          </li>
        ))}
      </ul>
    </section>
  );
}
