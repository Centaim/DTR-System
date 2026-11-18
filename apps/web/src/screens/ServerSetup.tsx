import { useState, type FormEvent } from "react";
import { setServerAddress } from "../lib/api";
import { useT } from "../lib/i18n";

/** First run of the mobile app: which agency server should it talk to? */
export function ServerSetup({ onDone }: { onDone: () => void }) {
  const { t } = useT();
  const [url, setUrl] = useState("https://");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");

  const save = async (ev: FormEvent) => {
    ev.preventDefault();
    setBusy(true);
    setError("");
    const address = url.trim().replace(/\/+$/, "");
    try {
      const res = await fetch(`${address}/api/health`);
      if (!res.ok) throw new Error("unhealthy");
      setServerAddress(address);
      onDone();
    } catch {
      setError(t.serverFailed);
    } finally {
      setBusy(false);
    }
  };

  return (
    <div className="shell">
      <header className="topbar"><span className="brand">{t.appName}</span></header>
      <main className="main narrow">
        <form className="panel form" onSubmit={save} noValidate>
          <h1 className="title">{t.serverTitle}</h1>
          <p className="muted">{t.serverHelp}</p>
          <label className="field">
            <span>{t.serverLabel}</span>
            <input type="url" inputMode="url" autoCapitalize="none" spellCheck={false} value={url} onChange={(e) => setUrl(e.target.value)} />
          </label>
          <div aria-live="polite">{error && <p className="msg bad">{error}</p>}</div>
          <button className="btn primary" type="submit" disabled={busy || url.trim().length < 10}>
            {busy ? t.kChecking : t.serverSave}
          </button>
        </form>
      </main>
    </div>
  );
}
