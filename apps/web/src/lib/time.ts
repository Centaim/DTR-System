const ZONE = "Asia/Manila";

const timeFmt = new Intl.DateTimeFormat("en-PH", {
  timeZone: ZONE,
  hour: "numeric",
  minute: "2-digit",
  hour12: true,
});

const clockFmt = new Intl.DateTimeFormat("en-US", {
  timeZone: ZONE,
  hour: "numeric",
  minute: "2-digit",
  second: "2-digit",
  hour12: true,
});

const dateFmt = (lang: string) =>
  new Intl.DateTimeFormat(lang === "fil" ? "fil-PH" : "en-PH", {
    timeZone: ZONE,
    weekday: "long",
    month: "long",
    day: "numeric",
    year: "numeric",
  });

export const formatTime = (d: Date | string | number) => timeFmt.format(new Date(d));

/** 12-hour clock to match Form 48: "7:02" large, ":09 PM" small. */
export function clockParts(ms: number): { hm: string; s: string } {
  const parts = clockFmt.formatToParts(new Date(ms));
  const get = (type: string) => parts.find((p) => p.type === type)?.value ?? "";
  return {
    hm: `${get("hour")}:${get("minute")}`,
    s: `:${get("second")} ${get("dayPeriod").toUpperCase()}`,
  };
}

export const formatDate = (ms: number, lang: string) => dateFmt(lang).format(new Date(ms));
