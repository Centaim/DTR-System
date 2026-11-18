import { useCallback, useEffect, useRef, useState } from "react";
import { ApiError, NetworkError, api, deviceId, type Today } from "./api";
import { queue, type QueueItem } from "./queue";

export const SLOTS = ["am_in", "am_out", "pm_in", "pm_out"] as const;

export type RowState = "synced" | "pending" | "empty";

export interface Row {
  slot: (typeof SLOTS)[number];
  state: RowState;
  at: string | null; // ISO time
}

export type ClockResult =
  | { kind: "synced"; slot: string; at: string }
  | { kind: "saved-offline"; slot: string; at: string }
  | { kind: "error"; code: string; message: string };

interface State {
  today: Today | null;
  items: QueueItem[];
  online: boolean;
  offsetMs: number; // server time minus device time
  sessionEnded: boolean;
}

export function useAttendance() {
  const [s, setS] = useState<State>({
    today: null,
    items: [],
    online: navigator.onLine,
    offsetMs: 0,
    sessionEnded: false,
  });
  const [busy, setBusy] = useState(false);
  const [freshSlot, setFreshSlot] = useState<string | null>(null);
  const flushing = useRef(false);

  const refresh = useCallback(async () => {
    const items = await queue.list();
    try {
      const today = await api.today();
      const offsetMs = Date.parse(today.server_time) - Date.now();
      setS((p) => ({ ...p, today, items, online: true, offsetMs, sessionEnded: false }));
      return today;
    } catch (e) {
      if (e instanceof ApiError && e.status === 401) {
        setS((p) => ({ ...p, items, sessionEnded: true }));
      } else {
        setS((p) => ({ ...p, items, online: false }));
      }
      return null;
    }
  }, []);

  const flush = useCallback(async () => {
    if (flushing.current) return;
    flushing.current = true;
    try {
      for (const item of (await queue.list()).filter((i) => i.status === "pending")) {
        try {
          await api.clock({
            key: item.key,
            photo: item.photo,
            clientTime: item.clientTime,
            offline: true,
            deviceId: item.deviceId,
          });
          await queue.remove(item.key);
        } catch (e) {
          if (e instanceof NetworkError) break;
          if (e instanceof ApiError && e.status === 401) {
            setS((p) => ({ ...p, sessionEnded: true }));
            break;
          }
          if (e instanceof ApiError && e.status >= 400 && e.status < 500) {
            await queue.put({ ...item, status: "rejected", reason: e.message });
            continue;
          }
          break; // server trouble: keep the item and retry later
        }
      }
    } finally {
      flushing.current = false;
      await refresh();
    }
  }, [refresh]);

  useEffect(() => {
    void refresh().then(() => flush());
    const onOnline = () => void flush();
    const onOffline = () => setS((p) => ({ ...p, online: false }));
    window.addEventListener("online", onOnline);
    window.addEventListener("offline", onOffline);
    const timer = window.setInterval(() => void flush(), 20_000);
    return () => {
      window.removeEventListener("online", onOnline);
      window.removeEventListener("offline", onOffline);
      window.clearInterval(timer);
    };
  }, [refresh, flush]);

  const pending = s.items.filter((i) => i.status === "pending");
  const rejected = s.items.filter((i) => i.status === "rejected");
  const logs = s.today?.logs ?? [];

  const rows: Row[] = SLOTS.map((slot, i) => {
    const log = logs[i];
    if (log) return { slot, state: "synced", at: log.recorded_at };
    const queued = pending[i - logs.length];
    if (queued) return { slot, state: "pending", at: queued.clientTime };
    return { slot, state: "empty", at: null };
  });

  const filled = logs.length + pending.length;
  const nextSlot = filled < SLOTS.length ? SLOTS[filled]! : null;

  const clock = useCallback(
    async (photo: Blob): Promise<ClockResult> => {
      setBusy(true);
      const key = crypto.randomUUID();
      const clientTime = new Date().toISOString();
      try {
        const { log } = await api.clock({
          key,
          photo,
          clientTime,
          offline: false,
          deviceId: deviceId(),
        });
        setFreshSlot(log.slot);
        await refresh();
        return { kind: "synced", slot: log.slot, at: log.recorded_at };
      } catch (e) {
        if (e instanceof NetworkError) {
          const slot = nextSlot ?? "am_in";
          await queue.put({
            key,
            photo,
            clientTime,
            deviceId: deviceId(),
            createdAt: Date.now(),
            status: "pending",
          });
          setS((p) => ({ ...p, online: false }));
          await refresh();
          return { kind: "saved-offline", slot, at: clientTime };
        }
        if (e instanceof ApiError) {
          if (e.status === 401) setS((p) => ({ ...p, sessionEnded: true }));
          return { kind: "error", code: e.code, message: e.message };
        }
        return { kind: "error", code: "unknown", message: "" };
      } finally {
        setBusy(false);
      }
    },
    [nextSlot, refresh],
  );

  const dismiss = useCallback(
    async (key: string) => {
      await queue.remove(key);
      await refresh();
    },
    [refresh],
  );

  return {
    rows,
    rejected,
    nextSlot,
    online: s.online,
    waiting: pending.length,
    offsetMs: s.offsetMs,
    sessionEnded: s.sessionEnded,
    loaded: s.today !== null,
    busy,
    freshSlot,
    clock,
    dismiss,
  };
}
