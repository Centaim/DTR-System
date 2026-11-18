import { useState } from "react";
import { useT } from "../lib/i18n";

/** The notice itself, shared by the app and the kiosk. */
export function PrivacyNotice({ onAccept }: { onAccept: () => Promise<void> }) {
  const { t } = useT();
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const parts = [
    [t.privacyCollectH, t.privacyCollect],
    [t.privacyWhyH, t.privacyWhy],
    [t.privacyWhoH, t.privacyWho],
    [t.privacyKeepH, t.privacyKeep],
    [t.privacyRightsH, t.privacyRights],
  ];
  const accept = async () => {
    setBusy(true);
    setError("");
    try {
      await onAccept();
    } catch {
      setError(t.errGeneric);
      setBusy(false);
    }
  };
  return (
    <section className="panel form privacy" aria-labelledby="pv-h">
      <h1 id="pv-h" className="title">
        {t.privacyTitle}
      </h1>
      <p className="muted">{t.privacyIntro}</p>
      {parts.map(([h, body]) => (
        <div key={h}>
          <h2 className="privacy-h">{h}</h2>
          <p>{body}</p>
        </div>
      ))}
      <div aria-live="polite">{error && <p className="msg bad">{error}</p>}</div>
      <button className="btn primary" onClick={accept} disabled={busy}>
        {t.privacyAccept}
      </button>
    </section>
  );
}
