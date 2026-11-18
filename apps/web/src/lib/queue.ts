// Offline queue in IndexedDB. Phase 0: photos are stored unencrypted on the
// device. Encryption at rest arrives with the Tauri secure-storage work.

export interface QueueItem {
  key: string;
  photo: Blob;
  clientTime: string;
  deviceId: string;
  latitude?: number;
  longitude?: number;
  createdAt: number;
  status: "pending" | "rejected";
  reason?: string;
}

const DB = "dtr-queue";
const STORE = "items";

function open(): Promise<IDBDatabase> {
  return new Promise((resolve, reject) => {
    const req = indexedDB.open(DB, 1);
    req.onupgradeneeded = () => req.result.createObjectStore(STORE, { keyPath: "key" });
    req.onsuccess = () => resolve(req.result);
    req.onerror = () => reject(req.error);
  });
}

async function run<T>(mode: IDBTransactionMode, fn: (s: IDBObjectStore) => IDBRequest<T>): Promise<T> {
  const db = await open();
  try {
    return await new Promise<T>((resolve, reject) => {
      const req = fn(db.transaction(STORE, mode).objectStore(STORE));
      req.onsuccess = () => resolve(req.result);
      req.onerror = () => reject(req.error);
    });
  } finally {
    db.close();
  }
}

export const queue = {
  async list(): Promise<QueueItem[]> {
    try {
      const all = await run("readonly", (s) => s.getAll() as IDBRequest<QueueItem[]>);
      return all.sort((a, b) => a.createdAt - b.createdAt);
    } catch {
      return [];
    }
  },
  put: (item: QueueItem) => run("readwrite", (s) => s.put(item)),
  remove: (key: string) => run("readwrite", (s) => s.delete(key)),
};
