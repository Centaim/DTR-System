export function shiftMonth(month: string, by: number): string {
  const [y = 0, m = 1] = month.split("-").map(Number);
  const d = new Date(Date.UTC(y, m - 1 + by, 1));
  return `${d.getUTCFullYear()}-${String(d.getUTCMonth() + 1).padStart(2, "0")}`;
}

export const currentMonth = () =>
  new Intl.DateTimeFormat("en-CA", { timeZone: "Asia/Manila" }).format(new Date()).slice(0, 7);

export function monthLabel(month: string): string {
  const [y = 0, m = 1] = month.split("-").map(Number);
  return new Intl.DateTimeFormat("en-PH", { timeZone: "UTC", month: "long", year: "numeric" }).format(
    new Date(Date.UTC(y, m - 1, 1)),
  );
}

export const to12 = (hhmm: string) => {
  if (!hhmm) return "";
  const [h = "0", m = "00"] = hhmm.split(":");
  return `${Number(h) % 12 || 12}:${m}`;
};
