import { useCallback, useEffect, useState, type FormEvent } from "react";
import { ApiError, api, type LegalHold, type SettingRow } from "../../lib/api";
import { useT, type Strings } from "../../lib/i18n";

const GROUPS = ["attendance", "security", "privacy", "flags"] as const;
const setLabel = (t: Strings, k: string) => (t[`set_${k}` as keyof Strings] as string) ?? k;
const groupLabel = (t: Strings, g: string) => t[`group_${g}` as keyof Strings] as string;

export function SettingsTab({ canEdit }: { canEdit: boolean }) {
  const { t } = useT();
  const [rows, setRows] = useState<SettingRow[] | null>(null);
  const [draft, setDraft] = useState<Record<string, string>>({});
  const [holds, setHolds] = useState<LegalHold[]>([]);
  const [hold, setHold] = useState({ employee_no: "", reason: "" });
  const [failed, setFailed] = useState(false);
  const [error, setError] = useState("");
  const [holdError, setHoldError] = useState("");
  const [saved, setSaved] = useState(false);

  const apply = (l: SettingRow[]) => {
    setRows(l);
    setDraft(Object.fromEntries(l.map((r) => [r.key, String(r.value)])));
  };
  const load = useCallback(() => {
    setFailed(false);
    api.admin.settings().then(apply).catch(() => setFailed(true));
    api.admin.legalHolds().then(setHolds).catch(() => setFailed(true));
  }, []);
  useEffect(load, [load]);

  const save = async (ev: FormEvent) => {
    ev.preventDefault();
    setError("");
    setSaved(false);
    try {
      apply(await api.admin.saveSettings(Object.fromEntries(Object.entries(draft).map(([k, v]) => [k, Number(v)]))));
      setSaved(true);
    } catch (e) {
      setError(e instanceof ApiError && e.message ? e.message : t.errGeneric);
    }
  };

  const addHold = async (ev: FormEvent) => {
    ev.preventDefault();
    setHoldError("");
    try {
      await api.admin.createLegalHold(hold.employee_no, hold.reason);
      setHold({ employee_no: "", reason: "" });
      load();
    } catch (e) {
      setHoldError(e instanceof ApiError && e.message ? e.message : t.errGeneric);
    }
  };

  if (failed) {
    return (
      <p className="msg bad" role="alert">
        {t.loadFailed} <button className="link-btn" onClick={load}>{t.retry}</button>
      </p>
    );
  }
  if (!rows) return <p className="muted" role="status">{t.photoLoading}</p>;

  return (
    <div className="admin-cols">
      <form className="panel form" onSubmit={save} noValidate>
        <h2 className="panel-h">{t.tabSettings}</h2>
        <p className="muted small">{t.settingsNote}</p>
        {GROUPS.map((g) => (
          <fieldset key={g} className="set-group" disabled={!canEdit}>
            <legend>{groupLabel(t, g)}</legend>
            {rows.filter((r) => r.group === g).map((r) => (
              <label key={r.key} className="field">
                <span>{setLabel(t, r.key)}</span>
                <input
                  type="number"
                  inputMode="numeric"
                  min={r.min}
                  max={r.max}
                  value={draft[r.key] ?? ""}
                  onChange={(e) => setDraft({ ...draft, [r.key]: e.target.value })}
                />
                <small className="muted">{t.rangeHint(r.min, r.max)}</small>
              </label>
            ))}
          </fieldset>
        ))}
        <div aria-live="polite">
          {error && <p className="msg bad">{error}</p>}
          {saved && <p className="msg ok">{t.settingsSaved}</p>}
        </div>
        {canEdit && <button className="btn primary" type="submit">{t.settingsSave}</button>}
      </form>

      <section className="panel form" aria-labelledby="hold-h">
        <h2 id="hold-h" className="panel-h">{t.holdsTitle}</h2>
        <p className="muted small">{t.holdsNote}</p>
        {holds.length === 0 && <p className="muted">{t.holdsEmpty}</p>}
        <ul className="rows flat">
          {holds.map((h) => (
            <li key={h.id} className="row">
              <span className="row-main">
                <span className="rail-slot">{h.name}</span>
                <span className="rail-state">{h.employee_no} · {h.reason}</span>
              </span>
              <button className="link-btn" onClick={() => api.admin.releaseLegalHold(h.id).finally(load)}>
                {t.holdRelease}
              </button>
            </li>
          ))}
        </ul>
        <form className="form" onSubmit={addHold} noValidate>
          <label className="field">
            <span>{t.holdEmployee}</span>
            <input value={hold.employee_no} onChange={(e) => setHold({ ...hold, employee_no: e.target.value })} />
          </label>
          <label className="field">
            <span>{t.holdReason}</span>
            <textarea rows={2} maxLength={300} value={hold.reason} onChange={(e) => setHold({ ...hold, reason: e.target.value })} />
          </label>
          <div aria-live="polite">{holdError && <p className="msg bad">{holdError}</p>}</div>
          <button className="btn quiet" type="submit" disabled={!hold.employee_no.trim() || hold.reason.trim().length < 5}>
            {t.holdAdd}
          </button>
        </form>
      </section>
    </div>
  );
}
