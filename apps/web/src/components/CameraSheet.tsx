import { useCallback, useEffect, useRef, useState } from "react";
import { Camera, RotateCcw, Check, X, TriangleAlert } from "lucide-react";
import { useT } from "../lib/i18n";
import { faceInFrame, preloadFaceCheck, sharpness } from "../lib/faceCheck";

type Phase =
  | { name: "starting" }
  | { name: "live" }
  | { name: "error"; message: "camDenied" | "camMissing" | "camFailed" }
  | { name: "review"; blob: Blob; url: string; dark: boolean; blurry: boolean; face: boolean | null | undefined };

const MAX_SIDE = 720;

function meanLuma(canvas: HTMLCanvasElement): number {
  const probe = document.createElement("canvas");
  probe.width = 24;
  probe.height = 24;
  const ctx = probe.getContext("2d");
  if (!ctx) return 255;
  ctx.drawImage(canvas, 0, 0, 24, 24);
  const { data } = ctx.getImageData(0, 0, 24, 24);
  let sum = 0;
  for (let i = 0; i < data.length; i += 4) {
    sum += 0.2126 * data[i]! + 0.7152 * data[i + 1]! + 0.0722 * data[i + 2]!;
  }
  return sum / (data.length / 4);
}

export function CameraSheet({
  onCancel,
  onConfirm,
  title,
  help,
}: {
  onCancel: () => void;
  onConfirm: (photo: Blob, faceSeen: boolean | null) => void;
  title?: string;
  help?: string;
}) {
  const { t } = useT();
  const [phase, setPhase] = useState<Phase>({ name: "starting" });
  const video = useRef<HTMLVideoElement>(null);
  const stream = useRef<MediaStream | null>(null);
  const dialog = useRef<HTMLDivElement>(null);

  const stop = useCallback(() => {
    stream.current?.getTracks().forEach((tr) => tr.stop());
    stream.current = null;
  }, []);

  const start = useCallback(async () => {
    setPhase({ name: "starting" });
    if (!navigator.mediaDevices?.getUserMedia) {
      setPhase({ name: "error", message: "camFailed" });
      return;
    }
    try {
      const s = await navigator.mediaDevices.getUserMedia({
        video: { facingMode: "user", width: { ideal: 960 }, height: { ideal: 1280 } },
        audio: false,
      });
      stream.current = s;
      if (video.current) {
        video.current.srcObject = s;
        await video.current.play().catch(() => undefined);
      }
      // "live" is set by the video's own playing event, once frames exist.
    } catch (e) {
      const name = e instanceof DOMException ? e.name : "";
      setPhase({
        name: "error",
        message:
          name === "NotAllowedError" || name === "SecurityError"
            ? "camDenied"
            : name === "NotFoundError" || name === "OverconstrainedError"
              ? "camMissing"
              : "camFailed",
      });
    }
  }, []);

  useEffect(() => {
    preloadFaceCheck();
    void start();
    return stop;
  }, [start, stop]);

  // Revoke the preview URL when it is replaced or the sheet closes.
  useEffect(() => {
    if (phase.name !== "review") return;
    const url = phase.url;
    return () => URL.revokeObjectURL(url);
  }, [phase]);

  // Keep focus inside the sheet and let Escape cancel.
  useEffect(() => {
    const el = dialog.current;
    if (!el) return;
    const onKey = (ev: KeyboardEvent) => {
      if (ev.key === "Escape") {
        onCancel();
        return;
      }
      if (ev.key !== "Tab") return;
      const f = [...el.querySelectorAll<HTMLElement>("button:not([disabled])")];
      if (f.length === 0) return;
      const first = f[0]!;
      const last = f[f.length - 1]!;
      if (ev.shiftKey && document.activeElement === first) {
        ev.preventDefault();
        last.focus();
      } else if (!ev.shiftKey && document.activeElement === last) {
        ev.preventDefault();
        first.focus();
      }
    };
    document.addEventListener("keydown", onKey);
    return () => document.removeEventListener("keydown", onKey);
  }, [onCancel]);

  useEffect(() => {
    dialog.current?.querySelector<HTMLElement>("[data-autofocus]")?.focus();
  }, [phase.name]);

  const capture = () => {
    const v = video.current;
    if (!v || v.videoWidth === 0) return;
    const scale = Math.min(1, MAX_SIDE / Math.max(v.videoWidth, v.videoHeight));
    const canvas = document.createElement("canvas");
    canvas.width = Math.round(v.videoWidth * scale);
    canvas.height = Math.round(v.videoHeight * scale);
    canvas.getContext("2d")?.drawImage(v, 0, 0, canvas.width, canvas.height);
    const dark = meanLuma(canvas) < 45;
    const blurry = !dark && sharpness(canvas) < 20;
    canvas.toBlob(
      (blob) => {
        if (!blob) return;
        stop();
        const url = URL.createObjectURL(blob);
        setPhase({ name: "review", blob, url, dark, blurry, face: undefined });
        void faceInFrame(canvas).then((face) =>
          setPhase((p) => (p.name === "review" && p.url === url ? { ...p, face } : p)),
        );
      },
      "image/jpeg",
      0.72,
    );
  };

  const retake = () => void start();

  return (
    <div
      className="sheet"
      role="dialog"
      aria-modal="true"
      aria-labelledby="cam-title"
      ref={dialog}
    >
      <div className="sheet-head">
        <h2 id="cam-title">{phase.name === "review" ? t.reviewTitle : (title ?? t.camTitle)}</h2>
        <p>{help ?? t.camHelp}</p>
      </div>

      <div className="viewfinder">
        <video
          ref={video}
          playsInline
          muted
          aria-label={t.camTitle}
          onPlaying={() => setPhase((p) => (p.name === "starting" ? { name: "live" } : p))}
          hidden={phase.name === "review" || phase.name === "error"}
        />
        {phase.name === "review" && <img src={phase.url} alt="" className="shot" />}
        {phase.name === "starting" && <p className="vf-note">{t.camStarting}</p>}
        {phase.name === "error" && (
          <p className="vf-note" role="alert">
            <TriangleAlert aria-hidden="true" size={20} /> {t[phase.message]}
          </p>
        )}
      </div>

      {phase.name === "review" && (phase.dark || phase.blurry || phase.face === false) && (
        <p className="warn-line" role="status">
          <TriangleAlert aria-hidden="true" size={18} />{" "}
          {phase.face === false ? t.noFace : phase.dark ? t.dark : t.blurry}
        </p>
      )}

      <div className="sheet-actions">
        {phase.name === "review" ? (
          <>
            <button
              className="btn primary"
              data-autofocus
              disabled={phase.face === undefined}
              onClick={() => onConfirm(phase.blob, phase.face ?? null)}
            >
              <Check aria-hidden="true" size={22} /> {phase.face === undefined ? t.photoChecking : t.usePhoto}
            </button>
            <button className="btn quiet" onClick={retake}>
              <RotateCcw aria-hidden="true" size={20} /> {t.retake}
            </button>
          </>
        ) : phase.name === "error" ? (
          <button className="btn primary" data-autofocus onClick={retake}>
            <RotateCcw aria-hidden="true" size={22} /> {t.camRetry}
          </button>
        ) : (
          <button
            className="btn primary"
            data-autofocus
            onClick={capture}
            disabled={phase.name !== "live"}
          >
            <Camera aria-hidden="true" size={22} /> {t.capture}
          </button>
        )}
        <button className="btn quiet cancel" onClick={onCancel}>
          <X aria-hidden="true" size={20} /> {t.cancel}
        </button>
      </div>
    </div>
  );
}
