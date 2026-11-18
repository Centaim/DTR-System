import { useEffect, useState } from "react";
import { LogIn, LogOut } from "lucide-react";
import type { Employee } from "../lib/api";
import { slotLabel, useT } from "../lib/i18n";
import { clockParts, formatDate, formatTime } from "../lib/time";
import { useAttendance, type ClockResult } from "../lib/useAttendance";
import { CameraSheet } from "../components/CameraSheet";
import { SeamRail } from "../components/SeamRail";
import { StatusPill } from "../components/StatusPill";
import { LangSwitch } from "../components/LangSwitch";

export function Home({
  employee,
  onSignOut,
}: {
  employee: Employee;
  onSignOut: (sessionEnded: boolean) => void;
}) {
  const { t, lang } = useT();
  const a = useAttendance();
  const [camera, setCamera] = useState(false);
  const [notice, setNotice] = useState<{ tone: "ok" | "warn" | "bad"; text: string } | null>(null);
  const [now, setNow] = useState(() => Date.now());

  useEffect(() => {
    const id = window.setInterval(() => setNow(Date.now()), 1000);
    return () => window.clearInterval(id);
  }, []);

  useEffect(() => {
    if (a.sessionEnded) onSignOut(true);
  }, [a.sessionEnded, onSignOut]);

  // Once queued logs reach the server, replace the "saved on this phone" notice.
  useEffect(() => {
    if (a.waiting === 0 && a.online) {
      setNotice((n) => (n?.tone === "warn" ? { tone: "ok", text: t.syncedNow } : n));
    }
  }, [a.waiting, a.online, t.syncedNow]);

  const official = now + a.offsetMs;
  const { hm, s } = clockParts(official);
  const isIn = a.nextSlot === "am_in" || a.nextSlot === "pm_in";

  const describe = (r: ClockResult) => {
    if (r.kind === "synced") {
      setNotice({ tone: "ok", text: t.clockedAt(slotLabel(t, r.slot), formatTime(r.at)) });
    } else if (r.kind === "saved-offline") {
      setNotice({ tone: "warn", text: t.savedOffline(slotLabel(t, r.slot), formatTime(r.at)) });
    } else {
      setNotice({ tone: "bad", text: r.message || t.errGeneric });
    }
  };

  const onPhoto = async (photo: Blob) => {
    setCamera(false);
    setNotice(null);
    describe(await a.clock(photo));
  };

  return (
    <div className="shell">
      <header className="topbar">
        <div className="brand-block">
          <span className="brand">{t.appName}</span>
          <span className="office">{employee.office}</span>
        </div>
        <StatusPill online={a.online} waiting={a.waiting} />
      </header>

      <main className="main grid">
        <section className="panel clock-panel" aria-labelledby="who">
          <h1 id="who" className="who">
            {employee.full_name}
            <span className="role">{employee.position}</span>
          </h1>

          <p className="date">{formatDate(official, lang)}</p>
          <p className="time" aria-label={`${hm}${s}`}>
            <span aria-hidden="true">
              {hm}
              <span className="secs">{s}</span>
            </span>
          </p>
          <p className="muted small">{a.online ? t.serverTime : t.deviceClock}</p>

          {a.nextSlot ? (
            <button
              className="btn primary punch"
              onClick={() => {
                setNotice(null);
                setCamera(true);
              }}
              disabled={a.busy || !a.loaded}
            >
              {isIn ? <LogIn aria-hidden="true" size={28} /> : <LogOut aria-hidden="true" size={28} />}
              <span className="punch-text">
                <span className="punch-main">{a.busy ? t.working : isIn ? t.clockIn : t.clockOut}</span>
                <span className="punch-sub">{slotLabel(t, a.nextSlot)}</span>
              </span>
            </button>
          ) : (
            a.loaded && <p className="msg ok">{t.dayComplete}</p>
          )}

          <div aria-live="polite" className="notice-slot">
            {notice && <p className={`msg ${notice.tone}`}>{notice.text}</p>}
          </div>
        </section>

        <SeamRail rows={a.rows} rejected={a.rejected} freshSlot={a.freshSlot} onDismiss={a.dismiss} />
      </main>

      <footer className="foot">
        <LangSwitch />
        <button className="btn quiet signout" onClick={() => onSignOut(false)}>
          {t.signOut}
        </button>
      </footer>

      {camera && <CameraSheet onCancel={() => setCamera(false)} onConfirm={onPhoto} />}
    </div>
  );
}
