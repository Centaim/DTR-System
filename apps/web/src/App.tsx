import { useCallback, useEffect, useMemo, useState } from "react";
import { ApiError, NetworkError, api, type Employee } from "./lib/api";
import { LangContext, initialLang, strings, type Lang } from "./lib/i18n";
import { Login } from "./screens/Login";
import { Home } from "./screens/Home";
import { Dtr } from "./screens/Dtr";
import { Admin } from "./screens/Admin";
import { ChangePin } from "./screens/ChangePin";
import { Requests } from "./screens/Requests";
import { Approvals } from "./screens/Approvals";
import { BottomNav, type Screen } from "./components/BottomNav";
import { More } from "./screens/More";
import { Alerts } from "./screens/Alerts";
import { Accomplishments } from "./screens/Accomplishments";
import { Events } from "./screens/Events";
import { Logbook } from "./screens/Logbook";
import { PrivacyNotice } from "./components/PrivacyNotice";

const EMP_KEY = "dtr.employee";

function cachedEmployee(): Employee | null {
  try {
    const raw = localStorage.getItem(EMP_KEY);
    return raw ? (JSON.parse(raw) as Employee) : null;
  } catch {
    return null;
  }
}

function cacheEmployee(e: Employee | null) {
  try {
    if (e) localStorage.setItem(EMP_KEY, JSON.stringify(e));
    else localStorage.removeItem(EMP_KEY);
  } catch {
    /* storage unavailable */
  }
}

export default function App() {
  const [lang, setLangState] = useState<Lang>(initialLang);
  const [employee, setEmployee] = useState<Employee | null>(null);
  const [booting, setBooting] = useState(api.hasToken());
  const [endedNotice, setEndedNotice] = useState(false);
  const [approvalCount, setApprovalCount] = useState(0);
  const [unread, setUnread] = useState(0);
  const clearUnread = useCallback(() => setUnread(0), []);
  const [screen, setScreen] = useState<Screen>("home");

  const setLang = useCallback((l: Lang) => {
    setLangState(l);
    document.documentElement.lang = l === "fil" ? "fil" : "en";
    try {
      localStorage.setItem("dtr.lang", l);
    } catch {
      /* storage unavailable */
    }
  }, []);

  const ctx = useMemo(() => ({ lang, setLang, t: strings(lang) }), [lang, setLang]);

  useEffect(() => {
    document.documentElement.lang = lang === "fil" ? "fil" : "en";
    if (!api.hasToken()) return;
    api
      .me()
      .then((e) => {
        cacheEmployee(e);
        setEmployee(e);
      })
      .catch((e) => {
        if (e instanceof NetworkError) setEmployee(cachedEmployee());
        else if (e instanceof ApiError && e.status === 401) api.setToken(null);
      })
      .finally(() => setBooting(false));
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const role = employee?.must_change_pin || !employee?.privacy_accepted ? undefined : employee.role;
  useEffect(() => {
    if (role === "supervisor" || role === "hr_admin" || role === "super_admin") {
      Promise.all([api.approvals.pending(), api.dtrReview.pending()])
        .then(([l, d]) => setApprovalCount(l.length + d.length))
        .catch(() => undefined);
    }
  }, [role]);

  const ready = Boolean(employee && !employee.must_change_pin && employee.privacy_accepted);
  useEffect(() => {
    if (!ready) return;
    const poll = () => api.notifications.list().then((n) => setUnread(n.unread)).catch(() => undefined);
    void poll();
    const id = window.setInterval(poll, 60_000);
    return () => window.clearInterval(id);
  }, [ready]);

  const signOut = useCallback((sessionEnded: boolean) => {
    api.setToken(null);
    cacheEmployee(null);
    setEmployee(null);
    setScreen("home");
    setEndedNotice(sessionEnded);
  }, []);

  const signedIn = useCallback((e: Employee) => {
    cacheEmployee(e);
    setEndedNotice(false);
    setEmployee(e);
  }, []);

  let view;
  if (booting) view = <div className="shell" aria-busy="true" />;
  else if (!employee)
    view = <Login onSignedIn={signedIn} notice={endedNotice ? ctx.t.sessionEnded : undefined} />;
  else if (employee.must_change_pin)
    view = (
      <ChangePin
        onDone={() => setEmployee({ ...employee, must_change_pin: false })}
        onSignOut={() => signOut(false)}
      />
    );
  else if (!employee.privacy_accepted)
    view = (
      <div className="shell">
        <main className="main narrow">
          <PrivacyNotice
            onAccept={async () => {
              await api.acceptPrivacy();
              signedIn({ ...employee, privacy_accepted: true });
            }}
          />
        </main>
      </div>
    );
  else if (screen === "more") view = <More employee={employee} approvals={approvalCount} onGo={setScreen} />;
  else if (screen === "alerts") view = <Alerts employee={employee} onRead={clearUnread} />;
  else if (screen === "accomplishments") view = <Accomplishments employee={employee} />;
  else if (screen === "events") view = <Events employee={employee} />;
  else if (screen === "logbook") view = <Logbook />;
  else if (screen === "admin") view = <Admin employee={employee} />;
  else if (screen === "dtr") view = <Dtr onSignOut={signOut} />;
  else if (screen === "requests") view = <Requests />;
  else if (screen === "approvals") view = <Approvals onCount={setApprovalCount} />;
  else view = <Home employee={employee} onSignOut={signOut} />;

  const showNav = !booting && employee !== null && !employee.must_change_pin && employee.privacy_accepted;

  return (
    <LangContext.Provider value={ctx}>
      <div className={showNav ? "has-nav" : undefined}>{view}</div>
      {showNav && employee && (
        <BottomNav screen={screen} approvals={approvalCount} unread={unread} onGo={setScreen} />
      )}
    </LangContext.Provider>
  );
}
