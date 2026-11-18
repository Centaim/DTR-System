import { useCallback, useEffect, useMemo, useState, type FormEvent } from "react";
import { Check, QrCode, ScanLine } from "lucide-react";
import { ApiError, NetworkError } from "../../lib/api";
import { LangContext, initialLang, slotLabel, strings, useT, type Lang, type Strings } from "../../lib/i18n";
import { kioskApi, type KioskInfo } from "../../lib/kioskApi";
import { clockParts, formatDate, formatTime } from "../../lib/time";
import { CameraSheet } from "../../components/CameraSheet";
import { LangSwitch } from "../../components/LangSwitch";
import { PrivacyNotice } from "../../components/PrivacyNotice";
import { KeyPad } from "./KeyPad";
import { ScanSheet } from "./ScanSheet";

type Step =
  | { name: "pair" }
  | { name: "idle" }
  | { name: "scan" }
  | { name: "pin"; no: string }
  | { name: "privacy" | "photo"; session: string; person: string; slot: string }
  | { name: "sending" }
  | { name: "done"; person: string; slot: string; at: string };

const IDLE_MS = 45_000;
const DONE_MS = 7_000;

function errorText(e: unknown, t: Strings): string {
  if (e instanceof NetworkError) return t.kNoConn;
  if (e instanceof ApiError) {
    const byCode: Record<string, string> = {
      invalid_credentials: t.kWrong,
      locked: t.kLocked,
      pin_change_required: t.kMustChange,
      day_complete: t.kDayComplete,
      too_soon: t.kTooSoon,
      session_expired: t.kExpired,
      rate_limited: t.kTooSoon,
    };
    return byCode[e.code] ?? (e.message || t.errGeneric);
  }
  return t.errGeneric;
}

export default function KioskApp() {
  const [lang, setLangState] = useState<Lang>(initialLang);
  const setLang = useCallback((l: Lang) => {
    setLangState(l);
    document.documentElement.lang = l === "fil" ? "fil" : "en";
    try {
      localStorage.setItem("dtr.lang", l);
    } catch {
      /* storage unavailable */
    }
  }, []);
  const ctx = useMemo(() => ({ lang, setLang, t: strings(lang) }), [lang, setLang]);
  return (
    <LangContext.Provider value={ctx}>
      <Kiosk />
    </LangContext.Provider>
  );
}

function Kiosk() {
  const { t, lang } = useT();
  const [info, setInfo] = useState<KioskInfo | null>(null);
  const [step, setStep] = useState<Step>(kioskApi.hasToken() ? { name: "idle" } : { name: "pair" });
  const [no, setNo] = useState("");
  const [pin, setPin] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const [now, setNow] = useState(Date.now());
  const [offset, setOffset] = useState(0);
  const [token, setToken] = useState("");

  const reset = useCallback(() => {
    setStep(kioskApi.hasToken() ? { name: "idle" } : { name: "pair" });
    setNo("");
    setPin("");
    setError("");
    setBusy(false);
  }, []);

  useEffect(() => {
    const id = window.setInterval(() => setNow(Date.now()), 1000);
    return () => window.clearInterval(id);
  }, []);

  // Load the kiosk's name and the official time. A rejected token sends us back to pairing.
  const pairing = step.name === "pair";
  useEffect(() => {
    if (!kioskApi.hasToken()) return;
    kioskApi
      .info()
      .then((i) => {
        setInfo(i);
        setOffset(Date.parse(i.server_time) - Date.now());
      })
      .catch((e) => {
        if (e instanceof ApiError && e.code === "kiosk_unregistered") setStep({ name: "pair" });
      });
  }, [pairing]);

  // Back to the start when nobody is using it, so one person's number is never left on screen.
  useEffect(() => {
    if (step.name === "idle" || step.name === "pair" || step.name === "sending") return;
    const ms = step.name === "done" ? DONE_MS : step.name === "privacy" ? 100_000 : IDLE_MS;
    const id = window.setTimeout(reset, ms);
    return () => window.clearTimeout(id);
  }, [step, no, pin, reset]);

  const pair = async (ev: FormEvent) => {
    ev.preventDefault();
    setBusy(true);
    setError("");
    try {
      setInfo(await kioskApi.pair(token));
      setToken("");
      setStep({ name: "idle" });
    } catch (e) {
      setError(e instanceof NetworkError ? t.kNoConn : t.kPairFail);
    } finally {
      setBusy(false);
    }
  };

  const verify = async () => {
    if (step.name !== "pin") return;
    setBusy(true);
    setError("");
    try {
      const r = await kioskApi.verify(step.no, pin);
      setPin("");
      setStep({ name: r.privacy_accepted ? "photo" : "privacy", session: r.session, person: r.name, slot: r.next_slot });
    } catch (e) {
      setPin("");
      setError(errorText(e, t));
      if (e instanceof ApiError && ["locked", "pin_change_required", "day_complete"].includes(e.code)) {
        window.setTimeout(reset, 5000);
      }
    } finally {
      setBusy(false);
    }
  };

  const send = async (photo: Blob) => {
    if (step.name !== "photo") return;
    const { session, person, slot } = step;
    setStep({ name: "sending" });
    try {
      const r = await kioskApi.clock(session, photo, crypto.randomUUID());
      setStep({ name: "done", person: r.name ?? person, slot: r.log.slot ?? slot, at: r.log.recorded_at });
    } catch (e) {
      setError(errorText(e, t));
      setStep({ name: "idle" });
    }
  };

  const { hm, s } = clockParts(now + offset);

  if (step.name === "pair") {
    return (
      <main className="kiosk">
        <form className="panel form kiosk-card" onSubmit={pair} noValidate>
          <h1 className="title">{t.kPair}</h1>
          <p className="muted">{t.kPairHelp}</p>
          <label className="field">
            <span>{t.kToken}</span>
            <input value={token} onChange={(e) => setToken(e.target.value)} spellCheck={false} autoComplete="off" required />
          </label>
          <div aria-live="polite">{error && <p className="msg bad">{error}</p>}</div>
          <button className="btn primary" type="submit" disabled={busy || !token.trim()}>
            {t.kPairBtn}
          </button>
        </form>
      </main>
    );
  }

  if (step.name === "privacy") {
    const next = step;
    return (
      <main className="kiosk">
        <PrivacyNotice
          onAccept={async () => {
            await kioskApi.acceptPrivacy(next.session);
            setStep({ ...next, name: "photo" });
          }}
        />
      </main>
    );
  }

  return (
    <main className="kiosk">
      <header className="kiosk-top">
        <div>
          <p className="brand">{info?.name ?? t.appName}</p>
          {info?.office && <p className="muted small">{info.office}</p>}
        </div>
        <LangSwitch />
      </header>

      <div className="kiosk-body">
        <section className="kiosk-clock" aria-label={t.serverTime}>
          <p className="date">{formatDate(now + offset, lang)}</p>
          <p className="time">
            {hm}
            <span className="secs">{s}</span>
          </p>
        </section>

        <section className="panel kiosk-card" aria-live="polite">
          {step.name === "idle" && (
            <form
              className="form"
              onSubmit={(e) => {
                e.preventDefault();
                if (no.trim()) {
                  setError("");
                  setStep({ name: "pin", no: no.trim().toUpperCase() });
                }
              }}
              noValidate
            >
              <h1 className="title">{t.kTitle}</h1>
              <p className="muted">{t.kEnterNoHelp}</p>
              <label className="field">
                <span>{t.kEnterNo}</span>
                <input
                  value={no}
                  onChange={(e) => setNo(e.target.value)}
                  autoCapitalize="characters"
                  autoComplete="off"
                  spellCheck={false}
                  autoFocus
                />
              </label>
              {error && <p className="msg bad">{error}</p>}
              <button className="btn primary kiosk-go" type="submit" disabled={!no.trim()}>
                {t.kNext}
              </button>
              <button className="btn quiet kiosk-go" type="button" onClick={() => setStep({ name: "scan" })}>
                <QrCode aria-hidden="true" size={24} /> {t.kScan}
              </button>
            </form>
          )}

          {step.name === "pin" && (
            <div className="form">
              <h1 className="title">{t.kPin}</h1>
              <p className="muted">{t.kPinFor(step.no)}</p>
              {busy && <p className="muted">{t.kChecking}</p>}
              {error && (
                <p className="msg bad" role="alert">
                  {error}
                </p>
              )}
              <KeyPad value={pin} onChange={setPin} onSubmit={verify} disabled={busy} />
              <button className="link-btn back" onClick={reset}>
                {t.kBack}
              </button>
            </div>
          )}

          {step.name === "photo" && (
            <div className="form">
              <h1 className="title">{t.kHello(step.person)}</h1>
              <p className="lead">{t.kNextLog(slotLabel(t, step.slot))}</p>
              <p className="muted">{t.kTakePhotoHelp}</p>
              <p className="muted small">{t.camHelp}</p>
            </div>
          )}

          {step.name === "sending" && (
            <div className="form" role="status">
              <ScanLine aria-hidden="true" size={40} />
              <h1 className="title">{t.kSending}</h1>
            </div>
          )}

          {step.name === "done" && (
            <div className="form done" role="status">
              <span className="done-mark" aria-hidden="true">
                <Check size={56} />
              </span>
              <h1 className="title">{t.kThanks}</h1>
              <p className="lead">{t.kDone(slotLabel(t, step.slot), formatTime(step.at))}</p>
              <p className="muted">{step.person}</p>
              <button className="btn quiet kiosk-go" onClick={reset}>
                {t.kReset}
              </button>
            </div>
          )}
        </section>
      </div>

      {step.name === "scan" && (
        <ScanSheet
          onScan={(n) => {
            setNo(n);
            setStep({ name: "pin", no: n.toUpperCase() });
          }}
          onCancel={reset}
        />
      )}
      {step.name === "photo" && (
        <CameraSheet
          title={t.kHello(step.person)}
          help={`${t.kNextLog(slotLabel(t, step.slot))}. ${t.camHelp}`}
          onCancel={reset}
          onConfirm={send}
        />
      )}
    </main>
  );
}
