import { API_BASE, ApiError, NetworkError } from "./api";

const KEY = "dtr.kiosk.token";

export const kioskToken = {
  get(): string | null {
    try {
      return localStorage.getItem(KEY);
    } catch {
      return null;
    }
  },
  set(t: string | null) {
    try {
      if (t) localStorage.setItem(KEY, t);
      else localStorage.removeItem(KEY);
    } catch {
      /* storage unavailable: pairing lasts until reload */
    }
  },
};

let memory: string | null = null;

async function call<T>(path: string, init: RequestInit = {}, token = memory ?? kioskToken.get()): Promise<T> {
  const headers = new Headers(init.headers);
  if (token) headers.set("X-Kiosk-Token", token);
  let res: Response;
  try {
    res = await fetch(API_BASE + path, { ...init, headers });
  } catch {
    throw new NetworkError("network");
  }
  if (!res.ok) {
    let code = "error";
    let message = "";
    try {
      const b = await res.json();
      code = b?.error?.code ?? code;
      message = b?.error?.message ?? "";
    } catch {
      /* non-JSON error */
    }
    const err = new ApiError(res.status, code, message);
    if (code === "kiosk_unregistered") {
      memory = null;
      kioskToken.set(null);
    }
    throw err;
  }
  return res.json() as Promise<T>;
}

export interface KioskInfo {
  name: string;
  office: string;
  server_time: string;
}

export const kioskApi = {
  hasToken: () => Boolean(memory ?? kioskToken.get()),
  /** Tries a token; keeps it only if the server accepts it. */
  async pair(token: string): Promise<KioskInfo> {
    const info = await call<KioskInfo>("/api/kiosk/info", {}, token.trim());
    memory = token.trim();
    kioskToken.set(memory);
    return info;
  },
  info: () => call<KioskInfo>("/api/kiosk/info"),
  verify: (employee_no: string, pin: string) =>
    call<{ session: string; name: string; next_slot: string; privacy_accepted: boolean }>("/api/kiosk/verify", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ employee_no, pin }),
    }),
  acceptPrivacy: (session: string) => {
    const form = new FormData();
    form.set("session", session);
    return call<{ ok: boolean }>("/api/kiosk/privacy", { method: "POST", body: form });
  },
  clock: (session: string, photo: Blob, key: string) => {
    const form = new FormData();
    form.set("session", session);
    form.set("idempotency_key", key);
    form.set("photo", photo, "photo.jpg");
    return call<{ log: { slot: string; recorded_at: string }; name: string }>("/api/kiosk/clock", {
      method: "POST",
      body: form,
    });
  },
};
