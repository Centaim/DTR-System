import { createContext, useContext } from "react";
import { adminEn } from "./i18n-admin";
import { requestsEn } from "./i18n-requests";
import { auditEn } from "./i18n-audit";
import { reportsEn } from "./i18n-reports";
import { peopleEn } from "./i18n-people";

export type Lang = "en" | "fil";

// Filipino strings are a first draft and need review by the agency before go-live.
const dict = {
  en: {
    appName: "DTR",
    online: "Online",
    offline: "Offline",
    waiting: (n: number) => `${n} waiting to sync`,
    signIn: "Sign in",
    signInHelp: "Use your employee number and the PIN from HR.",
    employeeNo: "Employee number",
    pin: "PIN",
    signingIn: "Signing in…",
    signOut: "Sign out",
    language: "Language",
    clockIn: "Clock in",
    clockOut: "Clock out",
    working: "Saving…",
    dayComplete: "All four logs are recorded for today.",
    next: "Next log",
    today: "Today",
    notYet: "Not yet",
    synced: "Synced",
    pending: "Saved on this phone, waiting to sync",
    rejected: "Not recorded",
    dismiss: "Dismiss",
    slot_am_in: "Morning arrival",
    slot_am_out: "Lunch departure",
    slot_pm_in: "Afternoon arrival",
    slot_pm_out: "End of day departure",
    clockedAt: (slot: string, time: string) => `${slot} recorded at ${time}.`,
    savedOffline: (slot: string, time: string) =>
      `${slot} saved at ${time} on this phone. It will sync when you are online.`,
    syncedNow: "Your saved logs are now synced.",
    viewDtr: "View my DTR",
    back: "Back",
    printDtr: "Print or save as PDF",
    prevMonth: "Previous month",
    nextMonth: "Next month",
    dtrLoading: "Loading your record…",
    dtrFailed: "Your record could not be loaded. Check your connection and try again.",
    dtrIncomplete: "Days marked with ! are missing a log. A supervisor must review them before certifying.",
    dtrSample: "Sample layout for the bid demo. Confirm it against the agency's official CSC Form No. 48.",
    serverTime: "Official time from the server",
    deviceClock: "This phone's clock. The server sets the official time when you sync.",
    camTitle: "Take your photo",
    camHelp: "Face the camera in good light.",
    camStarting: "Opening camera…",
    camDenied:
      "The camera is blocked. Allow camera access in your browser or phone settings, then try again.",
    camMissing: "No camera was found on this device.",
    camFailed: "The camera could not start. Close other apps using it and try again.",
    camRetry: "Try again",
    capture: "Take photo",
    cancel: "Cancel",
    usePhoto: "Use this photo",
    retake: "Retake",
    dark: "The photo is dark. Move to better light, or retake.",
    reviewTitle: "Check your photo",
    errNetwork: "No connection.",
    errGeneric: "Something went wrong. Try again.",
    sessionEnded: "Your session has ended. Sign in again.",
    demoNote: "Demo build: synthetic data only.",
    changePinTitle: "Choose a new PIN",
    changePinHelp: "Your PIN was set by HR. Choose your own before you continue.",
    currentPin: "Current PIN",
    newPin: "New PIN",
    confirmPin: "Enter the new PIN again",
    pinRules: "6 to 12 digits. Avoid repeated or counting digits such as 111111 or 123456.",
    pinMismatch: "The two new PINs do not match.",
    savePin: "Save new PIN",
    navLabel: "Main",
    navHome: "Home",
    navDtr: "Attendance",
    navRequests: "Requests",
    navApprovals: "Approvals",
    navAdmin: "HR admin",
    ...adminEn,
    ...requestsEn,
    ...auditEn,
    ...reportsEn,
    ...peopleEn,
  },
  fil: {
    appName: "DTR",
    online: "Online",
    offline: "Offline",
    waiting: (n: number) => `${n} naghihintay ma-sync`,
    signIn: "Mag-sign in",
    signInHelp: "Gamitin ang inyong employee number at ang PIN mula sa HR.",
    employeeNo: "Employee number",
    pin: "PIN",
    signingIn: "Nagsa-sign in…",
    signOut: "Mag-sign out",
    language: "Wika",
    clockIn: "Mag-clock in",
    clockOut: "Mag-clock out",
    working: "Sine-save…",
    dayComplete: "Naitala na ang lahat ng apat na log ngayong araw.",
    next: "Susunod na log",
    today: "Ngayon",
    notYet: "Wala pa",
    synced: "Na-sync na",
    pending: "Naka-save sa phone, naghihintay ma-sync",
    rejected: "Hindi naitala",
    dismiss: "Alisin",
    slot_am_in: "Pagdating sa umaga",
    slot_am_out: "Pag-alis sa tanghalian",
    slot_pm_in: "Pagdating sa hapon",
    slot_pm_out: "Pag-alis sa katapusan ng araw",
    clockedAt: (slot: string, time: string) => `Naitala ang ${slot} ng ${time}.`,
    savedOffline: (slot: string, time: string) =>
      `Na-save sa phone ang ${slot} ng ${time}. Mase-sync ito pag online na.`,
    syncedNow: "Na-sync na ang mga naka-save ninyong log.",
    viewDtr: "Tingnan ang aking DTR",
    back: "Bumalik",
    printDtr: "I-print o i-save bilang PDF",
    prevMonth: "Nakaraang buwan",
    nextMonth: "Susunod na buwan",
    dtrLoading: "Kinukuha ang inyong talaan…",
    dtrFailed: "Hindi makuha ang inyong talaan. Tingnan ang koneksyon at subukan ulit.",
    dtrIncomplete: "May kulang na log ang mga araw na may !. Dapat itong suriin ng supervisor bago i-certify.",
    dtrSample: "Sample na layout para sa bid demo. Ikumpara sa opisyal na CSC Form No. 48 ng ahensya.",
    serverTime: "Opisyal na oras mula sa server",
    deviceClock: "Oras ng phone na ito. Itatakda ng server ang opisyal na oras pag nag-sync.",
    camTitle: "Kunan ang inyong larawan",
    camHelp: "Humarap sa camera sa maliwanag na lugar.",
    camStarting: "Binubuksan ang camera…",
    camDenied:
      "Naka-block ang camera. Payagan ang camera sa settings ng browser o phone, saka subukan ulit.",
    camMissing: "Walang nakitang camera sa device na ito.",
    camFailed: "Hindi nagsimula ang camera. Isara ang ibang app na gumagamit nito at subukan ulit.",
    camRetry: "Subukan ulit",
    capture: "Kumuha ng larawan",
    cancel: "Kanselahin",
    usePhoto: "Gamitin ang larawang ito",
    retake: "Ulitin",
    dark: "Madilim ang larawan. Lumipat sa mas maliwanag na lugar, o ulitin.",
    reviewTitle: "Suriin ang inyong larawan",
    errNetwork: "Walang koneksyon.",
    errGeneric: "May nangyaring mali. Subukan ulit.",
    sessionEnded: "Tapos na ang inyong session. Mag-sign in ulit.",
    demoNote: "Demo build: gawa-gawang datos lamang.",
    changePinTitle: "Pumili ng bagong PIN",
    changePinHelp: "Itinakda ng HR ang inyong PIN. Pumili ng sarili ninyo bago magpatuloy.",
    currentPin: "Kasalukuyang PIN",
    newPin: "Bagong PIN",
    confirmPin: "Ilagay ulit ang bagong PIN",
    pinRules: "6 hanggang 12 numero. Iwasan ang magkakaparehong numero o sunod-sunod tulad ng 111111 o 123456.",
    pinMismatch: "Hindi magkapareho ang dalawang bagong PIN.",
    savePin: "I-save ang bagong PIN",
    navLabel: "Pangunahin",
    navHome: "Home",
    navDtr: "Attendance",
    navRequests: "Mga request",
    navApprovals: "Aprubahan",
    navAdmin: "HR admin",
  },
} as const;

export type Strings = (typeof dict)["en"];

export function strings(lang: Lang): Strings {
  // Filipino falls back to English for any string not yet translated.
  return (lang === "fil" ? { ...dict.en, ...dict.fil } : dict.en) as unknown as Strings;
}

export const LangContext = createContext<{
  lang: Lang;
  setLang: (l: Lang) => void;
  t: Strings;
}>({ lang: "en", setLang: () => {}, t: dict.en });

export const useT = () => useContext(LangContext);

export function slotLabel(t: Strings, slot: string): string {
  const key = `slot_${slot}` as keyof Strings;
  const v = t[key];
  return typeof v === "string" ? v : slot;
}

export function initialLang(): Lang {
  try {
    const saved = localStorage.getItem("dtr.lang");
    if (saved === "en" || saved === "fil") return saved;
  } catch {
    /* storage unavailable */
  }
  return navigator.language.toLowerCase().startsWith("fil") ? "fil" : "en";
}
