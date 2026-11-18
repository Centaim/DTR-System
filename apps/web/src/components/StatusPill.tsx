import { useT } from "../lib/i18n";

export function StatusPill({ online, waiting }: { online: boolean; waiting: number }) {
  const { t } = useT();
  return (
    <p className="pill" data-online={online} role="status">
      <span className="marker small" data-state={online ? "synced" : "pending"} aria-hidden="true" />
      <span>{online ? t.online : t.offline}</span>
      {waiting > 0 && <span className="pill-wait">· {t.waiting(waiting)}</span>}
    </p>
  );
}
