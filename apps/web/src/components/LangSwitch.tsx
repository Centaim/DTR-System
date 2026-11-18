import { useT, type Lang } from "../lib/i18n";

const OPTIONS: { value: Lang; label: string; name: string }[] = [
  { value: "en", label: "EN", name: "English" },
  { value: "fil", label: "FIL", name: "Filipino" },
];

export function LangSwitch() {
  const { lang, setLang, t } = useT();
  return (
    <div className="seg" role="group" aria-label={t.language}>
      {OPTIONS.map((o) => (
        <button
          key={o.value}
          type="button"
          className="seg-btn"
          aria-pressed={lang === o.value}
          aria-label={o.name}
          onClick={() => setLang(o.value)}
        >
          {o.label}
        </button>
      ))}
    </div>
  );
}
