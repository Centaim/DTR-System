import { useState, type FormEvent } from "react";
import { ApiError, NetworkError, api, type Employee } from "../lib/api";
import { useT } from "../lib/i18n";
import { LangSwitch } from "../components/LangSwitch";

export function Login({
  onSignedIn,
  notice,
}: {
  onSignedIn: (e: Employee) => void;
  notice?: string;
}) {
  const { t } = useT();
  const [no, setNo] = useState("");
  const [pin, setPin] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");

  const submit = async (ev: FormEvent) => {
    ev.preventDefault();
    setBusy(true);
    setError("");
    try {
      const res = await api.login(no.trim(), pin);
      api.setToken(res.token);
      onSignedIn(res.employee);
    } catch (e) {
      setPin("");
      if (e instanceof NetworkError) setError(t.errNetwork);
      else if (e instanceof ApiError && e.message) setError(e.message);
      else setError(t.errGeneric);
    } finally {
      setBusy(false);
    }
  };

  return (
    <div className="shell">
      <header className="topbar">
        <span className="brand">{t.appName}</span>
      </header>
      <main className="main narrow">
        <form className="panel form" onSubmit={submit} noValidate>
          <h1 className="title">{t.signIn}</h1>
          <p className="muted">{t.signInHelp}</p>
          {notice && (
            <p className="msg warn" role="status">
              {notice}
            </p>
          )}

          <label className="field">
            <span>{t.employeeNo}</span>
            <input
              value={no}
              onChange={(e) => setNo(e.target.value)}
              autoComplete="username"
              autoCapitalize="characters"
              spellCheck={false}
              required
            />
          </label>
          <label className="field">
            <span>{t.pin}</span>
            <input
              value={pin}
              onChange={(e) => setPin(e.target.value)}
              type="password"
              inputMode="numeric"
              autoComplete="current-password"
              required
            />
          </label>

          <div aria-live="polite">
            {error && <p className="msg bad">{error}</p>}
          </div>

          <button className="btn primary" type="submit" disabled={busy || !no || !pin}>
            {busy ? t.signingIn : t.signIn}
          </button>
        </form>
        <div className="foot-row">
          <LangSwitch />
          <span className="muted small">{t.demoNote}</span>
        </div>
      </main>
    </div>
  );
}
