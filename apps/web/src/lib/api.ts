const BASE = (import.meta.env.VITE_API_URL as string | undefined) ?? "";

export interface Employee {
  id: number;
  employee_no: string;
  full_name: string;
  position: string;
  office: string;
  role: string;
  must_change_pin: boolean;
}

export interface AdminEmployee {
  id: number;
  employee_no: string;
  full_name: string;
  position: string;
  office: string;
  role: string;
  employment: string;
  active: boolean;
  supervisor_id: number | null;
  schedule: string | null;
}

export interface EmployeeInput {
  employee_no?: string;
  full_name?: string;
  position?: string;
  office?: string;
  role?: string;
  employment?: string;
  active?: boolean;
  supervisor_id?: number;
}

export interface Schedule {
  id: number;
  name: string;
  am_start: string;
  am_end: string;
  pm_start: string;
  pm_end: string;
  is_default: boolean;
}

export interface RequestItem {
  id: number;
  kind: string;
  work_date: string;
  date_to: string | null;
  slot: string | null;
  time: string | null;
  reason: string;
  status: string;
  decision_note: string;
  created_at: string;
  employee?: string;
  employee_no?: string;
}

export interface RequestInput {
  kind: string;
  work_date: string;
  date_to?: string;
  slot?: string;
  time?: string;
  reason: string;
}

export interface AuditFlag {
  kind: string;
  detail: string;
}

export interface AuditItem {
  log_id: number;
  employee: string;
  employee_no: string;
  slot: string;
  recorded_at: string;
  channel: string;
  offline: boolean;
  device_id: string;
  flags: AuditFlag[];
  previous_log_id: number | null;
  review: { outcome: string; note: string; reviewer: string; reviewed_at: string } | null;
}

export interface Holiday {
  day: string;
  name: string;
}

export interface DashboardData {
  date: string;
  working_day: boolean;
  holiday: string;
  total: number;
  present: number;
  late: number;
  excused: number;
  not_in: number;
  pending_requests: number;
  open_flags: number;
  offline_today: number;
  offices: { office: string; total: number; present: number; late: number; excused: number; not_in: number }[];
  recent: { employee: string; slot: string; at: string; offline: boolean }[];
}

export interface SummaryRow {
  employee_no: string;
  name: string;
  office: string;
  position: string;
  present_days: number;
  late_days: number;
  absent_days: number;
  excused_days: number;
  undertime_min: number;
  incomplete_days: number;
  adjusted_days: number;
  habitual_late: boolean;
}

export interface SummaryData {
  month: string;
  working_days: number;
  habitual_threshold: number;
  rows: SummaryRow[];
}

export interface ImportRow {
  line: number;
  employee_no: string;
  full_name: string;
  position: string;
  office: string;
  role: string;
  employment: string;
  status: "ok" | "duplicate" | "invalid";
  message: string;
}

export interface ImportResult {
  rows: ImportRow[];
  summary: { ok: number; duplicate: number; invalid: number };
  created: { employee_no: string; full_name: string; temp_pin: string }[];
}

export interface ActivityRow {
  id: number;
  at: string;
  actor: string;
  action: string;
  target: string;
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

export interface DtrDay {
  day: number;
  weekday: string;
  am_in: string;
  am_out: string;
  pm_in: string;
  pm_out: string;
  undertime_min: number;
  incomplete: boolean;
  adjusted: boolean;
  mark: string;
}

export interface DtrResponse {
  employee: Employee;
  sheet: { month: string; days: DtrDay[]; total_undertime_min: number };
  official_hours: { am_start: string; am_end: string; pm_start: string; pm_end: string }[];
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

async function requestBlob(path: string): Promise<Blob> {
  const token = memoryToken ?? tokenStore.get();
  let res: Response;
  try {
    res = await fetch(BASE + path, { headers: token ? { Authorization: `Bearer ${token}` } : {} });
  } catch {
    throw new NetworkError("network");
  }
  if (!res.ok) throw new ApiError(res.status, "error", "");
  return res.blob();
}

const json = (method: string, body: unknown): RequestInit => ({
  method,
  headers: { "Content-Type": "application/json" },
  body: JSON.stringify(body),
});

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
  changePin: (current_pin: string, new_pin: string) =>
    request<{ ok: boolean }>("/api/me/pin", json("POST", { current_pin, new_pin })),

  requests: {
    mine: () => request<RequestItem[]>("/api/requests"),
    create: (body: RequestInput) => request<RequestItem>("/api/requests", json("POST", body)),
    cancel: (id: number) => request<{ ok: boolean }>(`/api/requests/${id}/cancel`, { method: "POST" }),
  },
  approvals: {
    pending: () => request<RequestItem[]>("/api/approvals/"),
    decide: (id: number, approve: boolean, note: string) =>
      request<RequestItem>(`/api/approvals/${id}/decide`, json("POST", { approve, note })),
  },

  reports: {
    dashboard: () => request<DashboardData>("/api/reports/dashboard"),
    summary: (month: string, office: string) =>
      request<SummaryData>(`/api/reports/summary?month=${month}&office=${encodeURIComponent(office)}`),
    csv: (month: string, office: string) =>
      requestBlob(`/api/reports/summary.csv?month=${month}&office=${encodeURIComponent(office)}`),
    holidays: (year: number) => request<Holiday[]>(`/api/reports/holidays?year=${year}`),
  },

  audit: {
    activity: () => request<ActivityRow[]>("/api/audit/activity"),
    list: (view: string, date: string) =>
      request<AuditItem[]>(`/api/audit/logs?view=${view}&date=${encodeURIComponent(date)}`),
    photo: (logId: number) => requestBlob(`/api/audit/logs/${logId}/photo`),
    review: (logId: number, outcome: "cleared" | "follow_up", note: string) =>
      request<{ ok: boolean }>(`/api/audit/logs/${logId}/review`, json("POST", { outcome, note })),
  },

  admin: {
    employees: (q: string) =>
      request<AdminEmployee[]>(`/api/admin/employees?q=${encodeURIComponent(q)}`),
    createEmployee: (body: EmployeeInput) =>
      request<{ employee: AdminEmployee; temp_pin: string }>("/api/admin/employees", json("POST", body)),
    updateEmployee: (id: number, body: EmployeeInput) =>
      request<AdminEmployee>(`/api/admin/employees/${id}`, json("PATCH", body)),
    resetPin: (id: number) =>
      request<{ temp_pin: string }>(`/api/admin/employees/${id}/reset-pin`, { method: "POST" }),
    importEmployees: (file: File, commit: boolean) => {
      const form = new FormData();
      form.set("file", file);
      return request<ImportResult>(`/api/admin/employees/import?commit=${commit}`, { method: "POST", body: form });
    },
    createHoliday: (body: Holiday) => request<Holiday>("/api/admin/holidays", json("POST", body)),
    deleteHoliday: (day: string) =>
      request<{ ok: boolean }>(`/api/admin/holidays/${day}`, { method: "DELETE" }),
    schedules: () => request<Schedule[]>("/api/admin/schedules"),
    createSchedule: (body: Omit<Schedule, "id" | "is_default">) =>
      request<Schedule>("/api/admin/schedules", json("POST", body)),
    assignSchedule: (id: number, schedule_id: number, effective_from: string) =>
      request<{ ok: boolean }>(`/api/admin/employees/${id}/schedule`, json("PUT", { schedule_id, effective_from })),
  },
  today: () => request<Today>("/api/attendance/today"),
  dtr: (month: string) => request<DtrResponse>(`/api/dtr?month=${encodeURIComponent(month)}`),

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
