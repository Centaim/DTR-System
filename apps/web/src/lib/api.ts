export const API_BASE = (import.meta.env.VITE_API_URL as string | undefined) ?? "";

export interface Employee {
  id: number;
  employee_no: string;
  full_name: string;
  position: string;
  office: string;
  role: string;
  must_change_pin: boolean;
  privacy_accepted: boolean;
  mfa_setup_required: boolean;
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
  clock_channels: number;
  email: string;
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
  clock_channels?: number;
  email?: string;
}

export interface Schedule {
  id: number;
  name: string;
  am_start: string;
  am_end: string;
  pm_start: string;
  pm_end: string;
  is_default: boolean;
  flexi: boolean;
  grace_minutes: number;
  required_minutes: number;
  latest_in: string;
  work_days: number;
  two_logs: boolean;
  overnight: boolean;
}

export interface Suspension {
  id: number;
  day: string;
  from_time: string;
  office: string;
  reason: string;
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
  has_attachment: boolean;
  source: string;
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

export interface KioskRow {
  id: number;
  name: string;
  office: string;
  active: boolean;
  last_seen_at: string | null;
}

export interface SettingRow {
  key: string;
  group: string;
  default: number;
  min: number;
  max: number;
  value: number;
}

export interface LegalHold {
  id: number;
  employee_no: string;
  name: string;
  reason: string;
  created_at: string;
}

export interface Geofence {
  id: number;
  name: string;
  latitude: number;
  longitude: number;
  radius_m: number;
  office: string;
}

export interface DeviceRequest {
  id: number;
  employee_no: string;
  name: string;
  device_id: string;
  requested_at: string;
}

export interface NotificationItem {
  id: number;
  kind: string;
  detail: string;
  read_at: string | null;
  created_at: string;
}

export interface Announcement {
  id: number;
  title: string;
  body: string;
  office: string;
  created_at: string;
  acked: boolean;
  ack_count: number;
  audience: number;
}

export interface Accomplishment {
  id: number;
  period_from: string;
  period_to: string;
  body: string;
  status: string;
  note: string;
  employee: string;
}

export interface EventItem {
  id: number;
  title: string;
  place: string;
  office: string;
  starts_at: string;
  ends_at: string;
  required: boolean;
  attended: boolean;
  count: number;
  audience: number;
}

export interface Attendee {
  employee_no: string;
  name: string;
  office: string;
  method: string | null;
  at: string | null;
}

export interface Delegation {
  id: number;
  to_name: string;
  from_date: string;
  to_date: string;
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
  slots: string[];
  location_wanted: boolean;
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
  late?: boolean;
  rest?: boolean;
  mark: string;
}

export interface DtrResponse {
  employee: Employee;
  sheet: { month: string; days: DtrDay[]; total_undertime_min: number };
  official_hours: { am_start: string; am_end: string; pm_start: string; pm_end: string }[];
  submission: DtrSubmission | null;
  can_certify: boolean;
}

export interface DtrSubmission {
  id: number;
  version: number;
  status: string;
  verify_code: string;
  certified_at: string;
  note: string;
}

export interface DtrPendingRow {
  id: number;
  name: string;
  employee_no: string;
  office: string;
  month: string;
  version: number;
}

export interface TrackerRow {
  employee_no: string;
  name: string;
  office: string;
  id: number;
  status: string;
  version: number;
}

export interface LoginResult {
  token?: string;
  refresh_token?: string;
  employee?: Employee;
  mfa_required?: boolean;
  mfa_token?: string;
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

const REFRESH_KEY = "dtr.refresh";

const refreshStore = {
  get(): string | null {
    try {
      return localStorage.getItem(REFRESH_KEY);
    } catch {
      return null;
    }
  },
  set(token: string | null) {
    try {
      if (token) localStorage.setItem(REFRESH_KEY, token);
      else localStorage.removeItem(REFRESH_KEY);
    } catch {
      /* storage unavailable: the session lasts until the access token expires */
    }
  },
};

let refreshing: Promise<boolean> | null = null;

/** Swaps the refresh token once, however many requests noticed the expiry at the same time. */
function renewSession(): Promise<boolean> {
  const current = refreshStore.get();
  if (!current) return Promise.resolve(false);
  refreshing ??= fetch(API_BASE + "/api/auth/refresh", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ refresh_token: current }),
  })
    .then(async (res) => {
      if (res.status === 401) {
        api.setToken(null);
        return false;
      }
      if (!res.ok) return false;
      const body = (await res.json()) as { token: string; refresh_token: string };
      api.setSession(body.token, body.refresh_token);
      return true;
    })
    .catch(() => false)
    .finally(() => {
      refreshing = null;
    });
  return refreshing;
}

/** Sends a request, renewing an expired access token once. */
async function send(path: string, init: RequestInit): Promise<Response> {
  const attempt = () => {
    const token = memoryToken ?? tokenStore.get();
    const headers = new Headers(init.headers);
    if (token) headers.set("Authorization", `Bearer ${token}`);
    return fetch(API_BASE + path, { ...init, headers });
  };
  let res: Response;
  try {
    res = await attempt();
    if (res.status === 401 && !path.startsWith("/api/auth/") && (await renewSession())) res = await attempt();
  } catch {
    throw new NetworkError("network");
  }
  return res;
}

async function request<T>(path: string, init: RequestInit = {}): Promise<T> {
  const res = await send(path, init);
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
  const res = await send(path, {});
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
    if (!token) refreshStore.set(null);
  },
  setSession(token: string, refresh: string) {
    memoryToken = token;
    tokenStore.set(token);
    refreshStore.set(refresh);
  },
  signOut() {
    const refresh = refreshStore.get();
    api.setToken(null);
    if (refresh) void fetch(API_BASE + "/api/auth/logout", { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ refresh_token: refresh }) }).catch(() => undefined);
  },
  hasToken: () => Boolean(memoryToken ?? tokenStore.get()),

  login: (employee_no: string, pin: string) =>
    request<LoginResult>("/api/auth/login", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ employee_no, pin }),
    }),

  mfaLogin: (mfa_token: string, code: string) =>
    request<LoginResult>("/api/auth/mfa", json("POST", { mfa_token, code })),
  mfaStart: () => request<{ secret: string; url: string }>("/api/me/mfa/start", { method: "POST" }),
  mfaConfirm: (code: string) => request<{ recovery_codes: string[] }>("/api/me/mfa/confirm", json("POST", { code })),
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

  certifyDtr: (month: string) => request<DtrSubmission>(`/api/dtr/certify?month=${month}`, { method: "POST" }),
  dtrReview: {
    pending: () => request<DtrPendingRow[]>("/api/dtr-review/"),
    open: (id: number) =>
      request<{ submission: DtrSubmission; dtr: Pick<DtrResponse, "employee" | "sheet" | "official_hours"> }>(`/api/dtr-review/${id}`),
    verify: (id: number) => request<{ ok: boolean }>(`/api/dtr-review/${id}/verify`, json("POST", {})),
    reject: (id: number, note: string) => request<{ ok: boolean }>(`/api/dtr-review/${id}/return`, json("POST", { note })),
  },
  dtrAdmin: {
    tracker: (month: string) => request<TrackerRow[]>(`/api/dtr-admin/tracker?month=${month}`),
    bulk: (month: string, office: string) =>
      request<DtrResponse[]>(`/api/dtr-admin/bulk?month=${month}&office=${encodeURIComponent(office)}`),
    reopen: (id: number, note: string) => request<{ ok: boolean }>(`/api/dtr-admin/${id}/reopen`, json("POST", { note })),
  },
  notifications: {
    list: () => request<{ items: NotificationItem[]; unread: number }>("/api/notifications"),
    markRead: () => request<{ ok: boolean }>("/api/notifications/read", { method: "POST" }),
  },
  announcements: {
    list: () => request<Announcement[]>("/api/announcements"),
    create: (body: { title: string; body: string; office: string }) =>
      request<{ ok: boolean }>("/api/announcements", json("POST", body)),
    ack: (id: number) => request<{ ok: boolean }>(`/api/announcements/${id}/ack`, { method: "POST" }),
  },
  accomplishments: {
    mine: () => request<Accomplishment[]>("/api/accomplishments"),
    pending: () => request<Accomplishment[]>("/api/accomplishments/pending"),
    create: (body: { period_from: string; period_to: string; body: string }) =>
      request<{ ok: boolean }>("/api/accomplishments", json("POST", body)),
    review: (id: number, accept: boolean, note: string) =>
      request<{ ok: boolean }>(`/api/accomplishments/${id}/review`, json("POST", { accept, note })),
  },
  events: {
    list: () => request<EventItem[]>("/api/events"),
    create: (body: { title: string; place: string; office: string; starts_at: string; ends_at: string; required: boolean }) =>
      request<{ ok: boolean }>("/api/events", json("POST", body)),
    checkIn: (id: number, photo: Blob) => {
      const form = new FormData();
      form.set("photo", photo, "photo.jpg");
      return request<{ ok: boolean }>(`/api/events/${id}/check-in`, { method: "POST", body: form });
    },
    attendance: (id: number) => request<Attendee[]>(`/api/events/${id}/attendance`),
    mark: (id: number, employee_no: string) =>
      request<{ ok: boolean }>(`/api/events/${id}/mark`, json("POST", { employee_no })),
  },
  delegations: {
    list: () => request<Delegation[]>("/api/delegations"),
    create: (body: { employee_no: string; from_date: string; to_date: string }) =>
      request<{ ok: boolean }>("/api/delegations", json("POST", body)),
    remove: (id: number) => request<{ ok: boolean }>(`/api/delegations/${id}`, { method: "DELETE" }),
  },
  logbook: (body: { employee_no: string; day: string; reason: string; entries: { slot: string; time: string }[] }) =>
    request<{ saved: number }>("/api/logbook", json("POST", body)),
  attachment: {
    upload: (id: number, file: File) => {
      const form = new FormData();
      form.set("file", file);
      return request<{ ok: boolean }>(`/api/requests/${id}/attachment`, { method: "POST", body: form });
    },
    open: (id: number) => requestBlob(`/api/requests/${id}/attachment`),
  },
  acceptPrivacy: () => request<{ ok: boolean }>("/api/me/privacy", { method: "POST" }),

  admin: {
    settings: () => request<SettingRow[]>("/api/admin/settings"),
    saveSettings: (values: Record<string, number>) =>
      request<SettingRow[]>("/api/admin/settings", json("PUT", values)),
    legalHolds: () => request<LegalHold[]>("/api/admin/legal-holds"),
    createLegalHold: (employee_no: string, reason: string) =>
      request<{ ok: boolean }>("/api/admin/legal-holds", json("POST", { employee_no, reason })),
    releaseLegalHold: (id: number) =>
      request<{ ok: boolean }>(`/api/admin/legal-holds/${id}/release`, { method: "POST" }),
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
    kiosks: () => request<KioskRow[]>("/api/admin/kiosks"),
    createKiosk: (name: string, office: string) =>
      request<{ kiosk: KioskRow; token: string }>("/api/admin/kiosks", json("POST", { name, office })),
    setKioskActive: (id: number, active: boolean) =>
      request<{ ok: boolean }>(`/api/admin/kiosks/${id}`, json("PATCH", { active })),
    suspensions: () => request<Suspension[]>("/api/admin/suspensions"),
    createSuspension: (body: Omit<Suspension, "id">) =>
      request<{ ok: boolean }>("/api/admin/suspensions", json("POST", body)),
    deleteSuspension: (id: number) => request<{ ok: boolean }>(`/api/admin/suspensions/${id}`, { method: "DELETE" }),
    geofences: () => request<Geofence[]>("/api/admin/geofences"),
    createGeofence: (body: Omit<Geofence, "id">) => request<{ ok: boolean }>("/api/admin/geofences", json("POST", body)),
    deleteGeofence: (id: number) => request<{ ok: boolean }>(`/api/admin/geofences/${id}`, { method: "DELETE" }),
    deviceRequests: () => request<DeviceRequest[]>("/api/admin/device-requests"),
    decideDevice: (id: number, approve: boolean) =>
      request<{ ok: boolean }>(`/api/admin/device-requests/${id}/${approve ? "approve" : "reject"}`, { method: "POST" }),
    resetMfa: (id: number) => request<{ ok: boolean }>(`/api/admin/employees/${id}/reset-mfa`, { method: "POST" }),
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
