const BASE = (import.meta.env.VITE_API_URL as string | undefined) ?? "";

export interface Employee {
  id: number;
  employee_no: string;
  full_name: string;
  position: string;
  office: string;
}

export interface LogEntry {
  id: number;
  slot: string;
  recorded_at: string;
  offline: boolean;
  channel: string;
}

export interface Today {
  work_date: string;
  logs: LogEntry[];
  next_slot: string | null;
  server_time: string;
}

export class NetworkError extends Error {}

export class ApiError extends Error {
  constructor(
    public status: number,
    public code: string,
    message: string,
  ) {
    super(message);
  }
}

const TOKEN_KEY = "dtr.token";

export const tokenStore = {
  get(): string | null {
    try {
      return localStorage.getItem(TOKEN_KEY);
    } catch {
      return null;
    }
  },
  set(token: string | null) {
    try {
      if (token) localStorage.setItem(TOKEN_KEY, token);
      else localStorage.removeItem(TOKEN_KEY);
    } catch {
      /* storage unavailable: the session lasts until reload */
    }
  },
};

let memoryToken: string | null = null;

async function request<T>(path: string, init: RequestInit = {}): Promise<T> {
  const token = memoryToken ?? tokenStore.get();
  const headers = new Headers(init.headers);
  if (token) headers.set("Authorization", `Bearer ${token}`);

  let res: Response;
  try {
    res = await fetch(BASE + path, { ...init, headers });
  } catch {
    throw new NetworkError("network");
  }
  if (!res.ok) {
    let code = "error";
    let message = "";
    try {
      const body = await res.json();
      code = body?.error?.code ?? code;
      message = body?.error?.message ?? "";
    } catch {
      /* non-JSON error body */
    }
    throw new ApiError(res.status, code, message);
  }
  return res.json() as Promise<T>;
}

export const api = {
  setToken(token: string | null) {
    memoryToken = token;
    tokenStore.set(token);
  },
  hasToken: () => Boolean(memoryToken ?? tokenStore.get()),

  login: (employee_no: string, pin: string) =>
    request<{ token: string; employee: Employee }>("/api/auth/login", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ employee_no, pin }),
    }),

  me: () => request<Employee>("/api/me"),
  today: () => request<Today>("/api/attendance/today"),

  clock: (p: {
    key: string;
    photo: Blob;
    clientTime: string;
    offline: boolean;
    deviceId: string;
    latitude?: number;
    longitude?: number;
  }) => {
    const form = new FormData();
    form.set("idempotency_key", p.key);
    form.set("photo", p.photo, "photo.jpg");
    form.set("client_time", p.clientTime);
    form.set("offline", String(p.offline));
    form.set("device_id", p.deviceId);
    if (p.latitude !== undefined) form.set("latitude", String(p.latitude));
    if (p.longitude !== undefined) form.set("longitude", String(p.longitude));
    return request<{ log: LogEntry; replay: boolean }>("/api/attendance/clock", {
      method: "POST",
      body: form,
    });
  },
};

/** Stable id for this browser or app install. Not a security control. */
export function deviceId(): string {
  const key = "dtr.device";
  try {
    let id = localStorage.getItem(key);
    if (!id) {
      id = crypto.randomUUID();
      localStorage.setItem(key, id);
    }
    return id;
  } catch {
    return "unknown";
  }
}
