import { useT } from "../../lib/i18n";

export function TempPin({ name, pin, onDone }: { name: string; pin: string; onDone: () => void }) {
  const { t } = useT();
  return (
    <section className="panel temp-pin" aria-labelledby="tp-h">
      <h2 id="tp-h" className="panel-h">
        {t.tempPinTitle}
      </h2>
      <p className="who">{name}</p>
      <p className="pin-big" aria-label={pin.split("").join(" ")}>
        {pin}
      </p>
      <p className="muted">{t.tempPinNote}</p>
      <button className="btn primary" onClick={onDone}>
        {t.done}
      </button>
    </section>
  );
}
