import { useCallback, useEffect, useMemo, useState } from "react";
import { ApiError, NetworkError, api, type Employee } from "./lib/api";
import { LangContext, initialLang, strings, type Lang } from "./lib/i18n";
import { Login } from "./screens/Login";
import { Home } from "./screens/Home";
import { Dtr } from "./screens/Dtr";
import { Admin } from "./screens/Admin";
import { ChangePin } from "./screens/ChangePin";

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
  const [screen, setScreen] = useState<"home" | "dtr" | "admin">("home");

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
  else if (screen === "admin") view = <Admin employee={employee} onBack={() => setScreen("home")} />;
  else if (screen === "dtr") view = <Dtr onBack={() => setScreen("home")} onSignOut={signOut} />;
  else view = <Home employee={employee} onSignOut={signOut} onOpenDtr={() => setScreen("dtr")} onOpenAdmin={() => setScreen("admin")} />;

  return <LangContext.Provider value={ctx}>{view}</LangContext.Provider>;
}
