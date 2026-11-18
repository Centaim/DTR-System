import { useEffect, useRef, useState } from "react";
import jsQR from "jsqr";
import { X } from "lucide-react";
import { useT } from "../../lib/i18n";

/** Reads the QR code on an ID card. The code holds "DTR:<employee number>". */
export function ScanSheet({ onScan, onCancel }: { onScan: (employeeNo: string) => void; onCancel: () => void }) {
  const { t } = useT();
  const video = useRef<HTMLVideoElement>(null);
  const [failed, setFailed] = useState(false);

  useEffect(() => {
    let stream: MediaStream | null = null;
    let raf = 0;
    let live = true;
    const canvas = document.createElement("canvas");
    const ctx = canvas.getContext("2d", { willReadFrequently: true });
    let last = 0;

    const tick = (now: number) => {
      raf = requestAnimationFrame(tick);
      const v = video.current;
      if (!v || !ctx || v.videoWidth === 0 || now - last < 180) return;
      last = now;
      const scale = Math.min(1, 480 / v.videoWidth);
      canvas.width = Math.round(v.videoWidth * scale);
      canvas.height = Math.round(v.videoHeight * scale);
      ctx.drawImage(v, 0, 0, canvas.width, canvas.height);
      const img = ctx.getImageData(0, 0, canvas.width, canvas.height);
      const code = jsQR(img.data, img.width, img.height, { inversionAttempts: "dontInvert" });
      if (code?.data.startsWith("DTR:")) {
        const no = code.data.slice(4).trim();
        if (no && no.length <= 30) {
          live = false;
          onScan(no);
        }
      }
    };

    navigator.mediaDevices
      ?.getUserMedia({ video: { facingMode: "user", width: { ideal: 640 } }, audio: false })
      .then(async (s) => {
        if (!live) return s.getTracks().forEach((x) => x.stop());
        stream = s;
        if (video.current) {
          video.current.srcObject = s;
          await video.current.play().catch(() => undefined);
        }
        raf = requestAnimationFrame(tick);
      })
      .catch(() => setFailed(true));
    if (!navigator.mediaDevices) setFailed(true);

    return () => {
      live = false;
      cancelAnimationFrame(raf);
      stream?.getTracks().forEach((x) => x.stop());
    };
  }, [onScan]);

  return (
    <div className="sheet" role="dialog" aria-modal="true" aria-labelledby="scan-title">
      <div className="sheet-head">
        <h2 id="scan-title">{t.kScanTitle}</h2>
        <p>{t.kScanHelp}</p>
      </div>
      <div className="viewfinder">
        <video ref={video} playsInline muted aria-label={t.kScanTitle} />
        {failed && <p className="vf-note" role="alert">{t.kNoCamera}</p>}
      </div>
      <div className="sheet-actions">
        <button className="btn quiet" onClick={onCancel} data-autofocus>
          <X aria-hidden="true" size={20} /> {t.cancel}
        </button>
      </div>
    </div>
  );
}
