import { useEffect, useState, type FormEvent } from "react";
import QRCode from "qrcode";
import { ApiError, api } from "../lib/api";
import { useT } from "../lib/i18n";

export function MfaSetup({ onDone }: { onDone: () => void }) {
  const { t } = useT();
  const [secret, setSecret] = useState("");
  const [qr, setQr] = useState("");
  const [code, setCode] = useState("");
  const [codes, setCodes] = useState<string[] | null>(null);
  const [error, setError] = useState("");

  useEffect(() => {
    api.mfaStart()
      .then(async (r) => {
        setSecret(r.secret);
        setQr(await QRCode.toDataURL(r.url, { margin: 1, width: 220 }));
      })
      .catch(() => setError(t.errGeneric));
  }, [t.errGeneric]);

  const confirm = async (ev: FormEvent) => {
    ev.preventDefault();
    setError("");
    try {
      setCodes((await api.mfaConfirm(code.trim())).recovery_codes);
    } catch (e) {
      setCode("");
      setError(e instanceof ApiError && e.message ? e.message : t.errGeneric);
    }
  };

  return (
    <div className="shell">
      <header className="topbar"><span className="brand">{t.appName}</span></header>
      <main className="main narrow">
        {codes ? (
          <section className="panel form">
            <h1 className="title">{t.mfaCodesTitle}</h1>
            <p className="muted">{t.mfaCodesHelp}</p>
            <ul className="codes">{codes.map((c) => <li key={c}>{c}</li>)}</ul>
            <button className="btn primary" onClick={onDone}>{t.mfaCodesDone}</button>
          </section>
        ) : (
          <form className="panel form" onSubmit={confirm} noValidate>
            <h1 className="title">{t.mfaSetupTitle}</h1>
            <p className="muted">{t.mfaSetupHelp}</p>
            {qr && <img className="mfa-qr" src={qr} alt="" width={220} height={220} />}
            <p className="muted small">{t.mfaManual}</p>
            <p className="token-box">{secret}</p>
            <label className="field">
              <span>{t.mfaCode}</span>
              <input value={code} inputMode="numeric" autoComplete="one-time-code" onChange={(e) => setCode(e.target.value)} />
            </label>
            <div aria-live="polite">{error && <p className="msg bad">{error}</p>}</div>
            <button className="btn primary" type="submit" disabled={code.trim().length < 6}>{t.mfaEnable}</button>
          </form>
        )}
      </main>
    </div>
  );
}
