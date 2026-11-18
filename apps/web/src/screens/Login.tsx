import { useState, type FormEvent } from "react";
import { ApiError, NetworkError, api, type Employee } from "../lib/api";
import { useT } from "../lib/i18n";
import { LangSwitch } from "../components/LangSwitch";

export function Login({
  onSignedIn,
  notice,
  onChangeServer,
}: {
  onSignedIn: (e: Employee) => void;
  notice?: string;
  onChangeServer?: () => void;
}) {
  const { t } = useT();
  const [no, setNo] = useState("");
  const [pin, setPin] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const [mfaToken, setMfaToken] = useState("");
  const [code, setCode] = useState("");

  const verify = async (ev: FormEvent) => {
    ev.preventDefault();
    setBusy(true);
    setError("");
    try {
      const res = await api.mfaLogin(mfaToken, code.trim());
      if (res.token && res.refresh_token && res.employee) {
        api.setSession(res.token, res.refresh_token);
        onSignedIn(res.employee);
      }
    } catch (e) {
      setCode("");
      if (e instanceof NetworkError) setError(t.errNetwork);
      else if (e instanceof ApiError && e.message) setError(e.message);
      else setError(t.errGeneric);
      if (e instanceof ApiError && e.code === "session_expired") setMfaToken("");
    } finally {
      setBusy(false);
    }
  };

  const submit = async (ev: FormEvent) => {
    ev.preventDefault();
    setBusy(true);
    setError("");
    try {
      const res = await api.login(no.trim(), pin);
      if (res.mfa_required && res.mfa_token) {
        setMfaToken(res.mfa_token);
      } else if (res.token && res.refresh_token && res.employee) {
        api.setSession(res.token, res.refresh_token);
        onSignedIn(res.employee);
      }
    } catch (e) {
      setPin("");
      if (e instanceof NetworkError) setError(t.errNetwork);
      else if (e instanceof ApiError && e.message) setError(e.message);
      else setError(t.errGeneric);
    } finally {
      setBusy(false);
    }
  };

  if (mfaToken) {
    return (
      <div className="shell">
        <header className="topbar"><span className="brand">{t.appName}</span></header>
        <main className="main narrow">
          <form className="panel form" onSubmit={verify} noValidate>
            <h1 className="title">{t.mfaLoginTitle}</h1>
            <p className="muted">{t.mfaLoginHelp}</p>
            <label className="field">
              <span>{t.mfaCode}</span>
              <input value={code} onChange={(e) => setCode(e.target.value)} autoComplete="one-time-code" autoFocus spellCheck={false} />
            </label>
            <div aria-live="polite">{error && <p className="msg bad">{error}</p>}</div>
            <button className="btn primary" type="submit" disabled={busy || !code.trim()}>{t.mfaVerify}</button>
          </form>
        </main>
      </div>
    );
  }

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
          {onChangeServer && <button type="button" className="link-btn" onClick={onChangeServer}>{t.serverChange}</button>}
          <span className="muted small">{t.demoNote}</span>
        </div>
      </main>
    </div>
  );
}
