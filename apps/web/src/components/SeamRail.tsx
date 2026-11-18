import { X } from "lucide-react";
import { slotLabel, useT } from "../lib/i18n";
import { formatTime } from "../lib/time";
import type { Row } from "../lib/useAttendance";
import type { QueueItem } from "../lib/queue";

/**
 * Today's four logs on one seam. The marker's form carries the state, so it
 * reads without colour: solid is synced, dashed is waiting, hollow is empty.
 */
export function SeamRail({
  rows,
  rejected,
  freshSlot,
  onDismiss,
}: {
  rows: Row[];
  rejected: QueueItem[];
  freshSlot: string | null;
  onDismiss: (key: string) => void;
}) {
  const { t } = useT();
  const stateText = (r: Row) =>
    r.state === "synced" ? t.synced : r.state === "pending" ? t.pending : t.notYet;

  return (
    <section className="panel" aria-labelledby="today-h">
      <h2 id="today-h" className="panel-h">
        {t.today}
      </h2>
      <ol className="rail">
        {rows.map((r) => (
          <li
            key={r.slot}
            className="rail-row"
            data-state={r.state}
            data-fresh={r.state === "synced" && r.slot === freshSlot ? "true" : undefined}
          >
            <span className="marker" aria-hidden="true" />
            <span className="rail-main">
              <span className="rail-slot">{slotLabel(t, r.slot)}</span>
              <span className="rail-state">{stateText(r)}</span>
            </span>
            <span className="rail-time">{r.at ? formatTime(r.at) : "—"}</span>
          </li>
        ))}
      </ol>

      {rejected.length > 0 && (
        <ul className="rejects">
          {rejected.map((r) => (
            <li key={r.key} className="reject-row">
              <span className="marker struck" aria-hidden="true" />
              <span className="rail-main">
                <span className="rail-slot">
                  {t.rejected} · {formatTime(r.clientTime)}
                </span>
                {r.reason && <span className="rail-state">{r.reason}</span>}
              </span>
              <button className="link-btn" onClick={() => onDismiss(r.key)}>
                <X aria-hidden="true" size={16} /> {t.dismiss}
              </button>
            </li>
          ))}
        </ul>
      )}
    </section>
  );
}
