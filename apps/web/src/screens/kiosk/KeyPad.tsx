import { useEffect } from "react";
import { Delete } from "lucide-react";
import { useT } from "../../lib/i18n";

const MAX = 12;

/** Big touch keys for the PIN. A hardware keyboard works too. */
export function KeyPad({
  value,
  onChange,
  onSubmit,
  disabled,
}: {
  value: string;
  onChange: (v: string) => void;
  onSubmit: () => void;
  disabled: boolean;
}) {
  const { t } = useT();

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (disabled) return;
      if (/^[0-9]$/.test(e.key) && value.length < MAX) onChange(value + e.key);
      else if (e.key === "Backspace") onChange(value.slice(0, -1));
      else if (e.key === "Enter" && value.length >= 6) onSubmit();
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [value, onChange, onSubmit, disabled]);

  const press = (d: string) => value.length < MAX && onChange(value + d);

  return (
    <div className="keypad-wrap">
      <p className="pin-dots" role="status" aria-label={`${value.length}`}>
        {Array.from({ length: Math.max(6, value.length) }, (_, i) => (
          <span key={i} className="pin-dot" data-filled={i < value.length} />
        ))}
      </p>
      <div className="keypad">
        {["1", "2", "3", "4", "5", "6", "7", "8", "9"].map((d) => (
          <button key={d} className="key" disabled={disabled} onClick={() => press(d)}>
            {d}
          </button>
        ))}
        <button className="key quiet-key" disabled={disabled || value === ""} onClick={() => onChange("")}>
          {t.kClear}
        </button>
        <button className="key" disabled={disabled} onClick={() => press("0")}>
          0
        </button>
        <button
          className="key quiet-key"
          disabled={disabled || value === ""}
          onClick={() => onChange(value.slice(0, -1))}
          aria-label="Backspace"
        >
          <Delete aria-hidden="true" size={28} />
        </button>
      </div>
      <button className="btn primary kiosk-go" disabled={disabled || value.length < 6} onClick={onSubmit}>
        {t.kEnter}
      </button>
    </div>
  );
}
