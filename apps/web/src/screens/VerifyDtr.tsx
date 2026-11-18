import { useEffect, useMemo, useState } from "react";
import { apiBase } from "../lib/api";
import { LangContext, initialLang, strings, useT, type Strings } from "../lib/i18n";
import { monthLabel } from "../lib/month";

interface Result {
  name: string;
  month: string;
  status: string;
  version: number;
  is_latest: boolean;
  fingerprint: string;
}

function Body({ code }: { code: string }) {
  const { t } = useT();
  const [r, setR] = useState<Result | "bad" | null>(null);
  useEffect(() => {
    fetch(`${apiBase()}/api/public/verify/${encodeURIComponent(code)}`)
      .then((x) => (x.ok ? x.json() : "bad"))
      .then(setR)
      .catch(() => setR("bad"));
  }, [code]);
  return (
    <div className="shell">
      <main className="main narrow">
        <section className="panel form" aria-live="polite">
          <h1 className="title">{t.verifyTitle}</h1>
          {r === null && <p className="muted">{t.dtrLoading}</p>}
          {r === "bad" && <p className="msg bad">{t.verifyBad}</p>}
          {r && r !== "bad" && (
            <>
              <p className={`msg ${r.is_latest ? "ok" : "warn"}`}>{r.is_latest ? t.verifyValid : t.verifyOld}</p>
              <dl className="facts">
                <dt>{t.verifyName}</dt>
                <dd>{r.name}</dd>
                <dt>{t.verifyMonth}</dt>
                <dd>{monthLabel(r.month)}</dd>
                <dt>{t.verifyStatus}</dt>
                <dd>
                  {t[`dtrSt_${r.status}` as keyof Strings] as string} · {t.dtrVersion(r.version)}
                </dd>
                <dt>{t.verifyFingerprint}</dt>
                <dd>{r.fingerprint}</dd>
              </dl>
            </>
          )}
        </section>
      </main>
    </div>
  );
}

export default function VerifyDtr({ code }: { code: string }) {
  const ctx = useMemo(() => {
    const lang = initialLang();
    return { lang, setLang: () => {}, t: strings(lang) };
  }, []);
  return (
    <LangContext.Provider value={ctx}>
      <Body code={code} />
    </LangContext.Provider>
  );
}
