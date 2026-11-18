import { useCallback, useEffect, useState } from "react";
import { RefreshCw } from "lucide-react";
import { api, type SystemStatus } from "../../lib/api";
import { useT } from "../../lib/i18n";
import { formatTime } from "../../lib/time";

const DAY = 26 * 3600 * 1000; // a daily job plus a little slack

function size(bytes: number): string {
  const units = ["B", "KB", "MB", "GB", "TB"];
  let v = bytes;
  let i = 0;
  while (v >= 1024 && i < units.length - 1) {
    v /= 1024;
    i++;
  }
  return `${v >= 10 || i === 0 ? v.toFixed(0) : v.toFixed(1)} ${units[i]}`;
}

const when = (iso: string) =>
  new Intl.DateTimeFormat("en-PH", { timeZone: "Asia/Manila", month: "short", day: "numeric" }).format(new Date(iso)) + ", " + formatTime(iso);

function Line({ tone, children }: { tone: "ok" | "bad" | "plain"; children: React.ReactNode }) {
  return (
    <li className="sys-line">
      <span className="marker small" data-state={tone === "ok" ? "synced" : tone === "bad" ? "pending" : undefined} aria-hidden="true" />
      <span>{children}</span>
    </li>
  );
}

export function SystemTab() {
  const { t } = useT();
  const [s, setS] = useState<SystemStatus | null>(null);
  const [failed, setFailed] = useState(false);

  const load = useCallback(() => {
    setFailed(false);
    api.compliance.system().then(setS).catch(() => setFailed(true));
  }, []);
  useEffect(() => {
    load();
    const id = window.setInterval(load, 60_000);
    return () => window.clearInterval(id);
  }, [load]);

  if (failed && !s) {
    return <p className="msg bad" role="alert">{t.loadFailed} <button className="link-btn" onClick={load}>{t.retry}</button></p>;
  }
  if (!s) return <p className="muted" role="status">{t.photoLoading}</p>;

  const b = s.backup;
  const okAt = b.last_ok_at ? Date.parse(b.last_ok_at) : 0;
  const failedLast = Boolean(b.last_failed_at && Date.parse(b.last_failed_at) > okAt);
  const stale = okAt > 0 && Date.now() - okAt > DAY;
  const backupBad = okAt === 0 || stale || failedLast;
  const lowDisk = s.storage.total_bytes > 0 && s.storage.free_bytes / s.storage.total_bytes < 0.1;
  const problems = [backupBad, lowDisk, !s.photos_encrypted, s.kiosks.online < s.kiosks.active].filter(Boolean).length;

  return (
    <div className="stack">
      <section className="panel">
        <div className="panel-bar">
          <p className={`lead sys-lead`} role="status">{problems ? t.sysAttention(problems) : t.sysHealthy}</p>
          <button className="btn quiet" onClick={load}><RefreshCw aria-hidden="true" size={20} /> {t.sysRefresh}</button>
        </div>
      </section>
      <div className="admin-cols">
        <div className="stack">
          <section className="panel">
            <h2 className="panel-h">{t.sysBackup}</h2>
            <ul className="sys-list">
              {okAt > 0 && <Line tone={stale ? "bad" : "ok"}>{t.sysBackupOk(when(b.last_ok_at!), size(b.bytes))}</Line>}
              {okAt === 0 && <Line tone="bad">{t.sysBackupNone}</Line>}
              {stale && <Line tone="bad">{t.sysBackupStale}</Line>}
              {failedLast && <Line tone="bad">{t.sysBackupFailed}</Line>}
            </ul>
          </section>
          <section className="panel">
            <h2 className="panel-h">{t.sysStorage}</h2>
            <ul className="sys-list">
              <Line tone={lowDisk ? "bad" : "ok"}>{t.sysStorageFree(size(s.storage.free_bytes), size(s.storage.total_bytes))}</Line>
              {lowDisk && <Line tone="bad">{t.sysStorageLow}</Line>}
              <Line tone="plain">{t.sysPhotos(s.storage.photo_files, size(s.storage.photo_bytes))}</Line>
              <Line tone="plain">{t.sysDatabase(size(s.storage.database_bytes))}</Line>
              <Line tone={s.photos_encrypted ? "ok" : "bad"}>{s.photos_encrypted ? t.sysEncrypted : t.sysNotEncrypted}</Line>
            </ul>
          </section>
        </div>
        <div className="stack">
          <section className="panel">
            <h2 className="panel-h">{t.sysKiosks}</h2>
            <ul className="sys-list">
              <Line tone={s.kiosks.online < s.kiosks.active ? "bad" : "ok"}>{t.sysKioskCount(s.kiosks.online, s.kiosks.active)}</Line>
            </ul>
          </section>
          <section className="panel">
            <h2 className="panel-h">{t.sysJobs}</h2>
            <ul className="sys-list">
              <Line tone="plain">{s.jobs.retention_last_run ? t.sysJobRetention(when(s.jobs.retention_last_run)) : t.sysJobNever}</Line>
              <Line tone="plain">{s.jobs.reminders_last_run ? t.sysJobReminders(when(s.jobs.reminders_last_run)) : t.sysJobNever}</Line>
            </ul>
          </section>
          <section className="panel">
            <h2 className="panel-h">{t.sysQueues}</h2>
            <ul className="sys-list">
              <Line tone="plain">{t.sysQueueRequests(s.queues.requests)}</Line>
              <Line tone="plain">{t.sysQueueDtrs(s.queues.dtrs)}</Line>
              <Line tone="plain">{t.sysQueueFlags(s.queues.flags)}</Line>
              <Line tone="plain">{t.sysQueueDevices(s.queues.devices)}</Line>
            </ul>
          </section>
        </div>
      </div>
    </div>
  );
}
