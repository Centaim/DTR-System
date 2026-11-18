// Offline queue in IndexedDB. Photos are sealed with AES-256-GCM under a key the
// browser holds as non-extractable, so a copy of the database cannot be opened.
// Each item belongs to the employee who made it, so a shared phone never syncs
// one person's log under another person's sign-in.

export interface QueueItem {
  key: string;
  employeeId: number;
  sealed: ArrayBuffer | null; // cleared once a log is rejected
  iv: Uint8Array;
  photoType: string;
  clientTime: string;
  deviceId: string;
  latitude?: number;
  longitude?: number;
  faceSeen?: boolean | null;
  createdAt: number;
  status: "pending" | "rejected";
  reason?: string;
}

export type NewQueueItem = Omit<QueueItem, "sealed" | "iv" | "photoType" | "status" | "reason"> & { photo: Blob };

const DB = "dtr-queue";
const STORE = "items";
const KEYS = "keys";

function open(): Promise<IDBDatabase> {
  return new Promise((resolve, reject) => {
    const req = indexedDB.open(DB, 2);
    req.onupgradeneeded = () => {
      const db = req.result;
      // Items from the older, unencrypted format are not carried over.
      if (db.objectStoreNames.contains(STORE)) db.deleteObjectStore(STORE);
      db.createObjectStore(STORE, { keyPath: "key" });
      if (!db.objectStoreNames.contains(KEYS)) db.createObjectStore(KEYS);
    };
    req.onsuccess = () => resolve(req.result);
    req.onerror = () => reject(req.error);
  });
}

async function run<T>(store: string, mode: IDBTransactionMode, fn: (s: IDBObjectStore) => IDBRequest<T>): Promise<T> {
  const db = await open();
  try {
    return await new Promise<T>((resolve, reject) => {
      const req = fn(db.transaction(store, mode).objectStore(store));
      req.onsuccess = () => resolve(req.result);
      req.onerror = () => reject(req.error);
    });
  } finally {
    db.close();
  }
}

async function photoKey(): Promise<CryptoKey> {
  const found = await run<CryptoKey | undefined>(KEYS, "readonly", (s) => s.get("photo"));
  if (found) return found;
  const made = await crypto.subtle.generateKey({ name: "AES-GCM", length: 256 }, false, ["encrypt", "decrypt"]);
  // add() fails if another tab won the race; the stored key is then the one to use.
  await run(KEYS, "readwrite", (s) => s.add(made, "photo")).catch(() => undefined);
  return (await run<CryptoKey | undefined>(KEYS, "readonly", (s) => s.get("photo"))) ?? made;
}

export const queue = {
  /** False where the browser offers no encryption, such as a plain http address. */
  canSeal: () => typeof crypto !== "undefined" && Boolean(crypto.subtle),

  async list(employeeId: number): Promise<QueueItem[]> {
    try {
      const all = await run<QueueItem[]>(STORE, "readonly", (s) => s.getAll());
      return all.filter((i) => i.employeeId === employeeId).sort((a, b) => a.createdAt - b.createdAt);
    } catch {
      return [];
    }
  },

  async add({ photo, ...item }: NewQueueItem): Promise<void> {
    const iv = crypto.getRandomValues(new Uint8Array(12));
    const sealed = await crypto.subtle.encrypt({ name: "AES-GCM", iv }, await photoKey(), await photo.arrayBuffer());
    const row: QueueItem = { ...item, sealed, iv, photoType: photo.type, status: "pending" };
    await run(STORE, "readwrite", (s) => s.put(row));
  },

  /** Opens a queued photo just long enough to upload it. */
  async photo(item: QueueItem): Promise<Blob> {
    if (!item.sealed) throw new Error("photo removed");
    const iv = new Uint8Array(item.iv);
    const plain = await crypto.subtle.decrypt({ name: "AES-GCM", iv }, await photoKey(), item.sealed);
    return new Blob([plain], { type: item.photoType });
  },

  /** Keeps the note about a refused log but deletes its photo. */
  reject: (item: QueueItem, reason: string) =>
    run(STORE, "readwrite", (s) => s.put({ ...item, sealed: null, status: "rejected", reason })),

  remove: (key: string) => run(STORE, "readwrite", (s) => s.delete(key)),
};
