import { useCallback, useEffect, useState, type FormEvent } from "react";
import { ApiError, api, type KioskRow } from "../../lib/api";
import { useT } from "../../lib/i18n";

function seen(iso: string | null, t: ReturnType<typeof useT>["t"]): string {
  if (!iso) return t.kioskNever;
  const mins = (Date.now() - Date.parse(iso)) / 60000;
  if (mins < 2) return t.kioskOnline;
  const when = new Intl.DateTimeFormat("en-PH", {
    timeZone: "Asia/Manila", month: "short", day: "numeric", hour: "numeric", minute: "2-digit",
  }).format(new Date(iso));
  return t.kioskLastSeen(when);
}

export function KiosksTab() {
  const { t } = useT();
  const [list, setList] = useState<KioskRow[] | null>(null);
  const [failed, setFailed] = useState(false);
  const [name, setName] = useState("");
  const [office, setOffice] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const [token, setToken] = useState<{ name: string; token: string } | null>(null);

  const load = useCallback(() => {
    setFailed(false);
    api.admin.kiosks().then(setList).catch(() => setFailed(true));
  }, []);
  useEffect(() => {
    load();
    const id = window.setInterval(load, 30_000);
    return () => window.clearInterval(id);
  }, [load]);

  const create = async (ev: FormEvent) => {
    ev.preventDefault();
    setBusy(true);
    setError("");
    try {
      const r = await api.admin.createKiosk(name, office);
      setToken({ name: r.kiosk.name, token: r.token });
      setName("");
      setOffice("");
      load();
    } catch (e) {
      setError(e instanceof ApiError && e.message ? e.message : t.errGeneric);
    } finally {
      setBusy(false);
    }
  };

  const toggle = async (k: KioskRow) => {
    try {
      await api.admin.setKioskActive(k.id, !k.active);
    } catch {
      /* the reload shows the true state */
    }
    load();
  };

  return (
    <div className="admin-cols">
      <section className="panel" aria-labelledby="ks-h">
        <h2 id="ks-h" className="panel-h">
          {t.tabKiosks}
        </h2>
        <p className="muted small">{t.kioskNote}</p>
        {failed && (
          <p className="msg bad" role="alert">
            {t.loadFailed} <button className="link-btn" onClick={load}>{t.retry}</button>
          </p>
        )}
        {list && list.length === 0 && <p className="muted">{t.noKiosks}</p>}
        <ul className="rows flat">
          {list?.map((k) => (
            <li key={k.id} className="row">
              <span className="row-main">
                <span className="rail-slot">
                  {k.name} {!k.active && <span className="tag bad">{t.kioskInactive}</span>}
                </span>
                <span className="rail-state">
                  {k.office || "—"} · {seen(k.last_seen_at, t)}
                </span>
              </span>
              <button className="link-btn" onClick={() => toggle(k)}>
                {k.active ? t.kioskDeactivate : t.kioskReactivate}
              </button>
            </li>
          ))}
        </ul>
      </section>

      <div className="stack">
        {token && (
          <section className="panel temp-pin" aria-labelledby="kt-h">
            <h2 id="kt-h" className="panel-h">
              {t.kioskTokenTitle}
            </h2>
            <p className="who">{token.name}</p>
            <p className="token-box">{token.token}</p>
            <p className="muted">{t.kioskTokenNote}</p>
            <p className="muted small">{`${window.location.origin}/kiosk`}</p>
            <button className="btn primary" onClick={() => setToken(null)}>
              {t.done}
            </button>
          </section>
        )}

        <form className="panel form" onSubmit={create} noValidate>
          <h2 className="panel-h">{t.registerKiosk}</h2>
          <label className="field">
            <span>{t.kioskName}</span>
            <input value={name} onChange={(e) => setName(e.target.value)} maxLength={60} required />
          </label>
          <label className="field">
            <span>{t.kioskOffice}</span>
            <input value={office} onChange={(e) => setOffice(e.target.value)} maxLength={80} />
          </label>
          <div aria-live="polite">{error && <p className="msg bad">{error}</p>}</div>
          <button className="btn primary" type="submit" disabled={busy || !name.trim()}>
            {busy ? t.saving : t.registerKiosk}
          </button>
        </form>
      </div>
    </div>
  );
}
