import { useState, type FormEvent } from "react";
import { ApiError, NetworkError, api } from "../lib/api";
import { useT } from "../lib/i18n";

export function ChangePin({ onDone, onSignOut }: { onDone: () => void; onSignOut: () => void }) {
  const { t } = useT();
  const [current, setCurrent] = useState("");
  const [next, setNext] = useState("");
  const [again, setAgain] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");

  const submit = async (ev: FormEvent) => {
    ev.preventDefault();
    if (next !== again) {
      setError(t.pinMismatch);
      return;
    }
    setBusy(true);
    setError("");
    try {
      await api.changePin(current, next);
      onDone();
    } catch (e) {
      if (e instanceof NetworkError) setError(t.errNetwork);
      else if (e instanceof ApiError && e.message) setError(e.message);
      else setError(t.errGeneric);
    } finally {
      setBusy(false);
    }
  };

  const pinField = (label: string, value: string, set: (v: string) => void, auto: string) => (
    <label className="field">
      <span>{label}</span>
      <input
        type="password"
        inputMode="numeric"
        autoComplete={auto}
        value={value}
        onChange={(e) => set(e.target.value)}
        required
      />
    </label>
  );

  return (
    <div className="shell">
      <header className="topbar">
        <span className="brand">{t.appName}</span>
      </header>
      <main className="main narrow">
        <form className="panel form" onSubmit={submit} noValidate>
          <h1 className="title">{t.changePinTitle}</h1>
          <p className="muted">{t.changePinHelp}</p>
          {pinField(t.currentPin, current, setCurrent, "current-password")}
          {pinField(t.newPin, next, setNext, "new-password")}
          {pinField(t.confirmPin, again, setAgain, "new-password")}
          <p className="muted small">{t.pinRules}</p>
          <div aria-live="polite">{error && <p className="msg bad">{error}</p>}</div>
          <button className="btn primary" type="submit" disabled={busy || !current || !next || !again}>
            {busy ? t.working : t.savePin}
          </button>
        </form>
        <div className="foot-row">
          <button type="button" className="btn quiet" onClick={onSignOut}>
            {t.signOut}
          </button>
        </div>
      </main>
    </div>
  );
}
