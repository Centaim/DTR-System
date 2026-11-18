// Checks only that a face is present in the photo. It runs on the device, keeps
// nothing, and never identifies or compares anyone.
type FaceApi = typeof import("@vladmandic/face-api");

let ready: Promise<FaceApi | null> | null = null;

function load(): Promise<FaceApi | null> {
  ready ??= import("@vladmandic/face-api")
    .then(async (faceapi) => {
      await (faceapi.tf as unknown as { ready(): Promise<void> }).ready();
      await faceapi.nets.tinyFaceDetector.loadFromUri("/models");
      return faceapi;
    })
    .catch(() => {
      ready = null; // try again next time, for example once back online
      return null;
    });
  return ready;
}

/** Starts loading early so the check is quick after the photo is taken. */
export function preloadFaceCheck() {
  void load();
}

const LIMIT_MS = 5000;

/** True or false when the check ran; null when this device could not run it in time. */
export async function faceInFrame(canvas: HTMLCanvasElement): Promise<boolean | null> {
  const check = (async () => {
    const faceapi = await load();
    if (!faceapi) return null;
    const found = await faceapi.detectSingleFace(
      canvas,
      new faceapi.TinyFaceDetectorOptions({ inputSize: 224, scoreThreshold: 0.4 }),
    );
    return Boolean(found);
  })().catch(() => null);
  const timeout = new Promise<null>((resolve) => window.setTimeout(() => resolve(null), LIMIT_MS));
  return Promise.race([check, timeout]);
}

/** Edge strength of the photo, on the same scale the server uses for its blur flag. */
export function sharpness(canvas: HTMLCanvasElement): number {
  const w = 160;
  const h = Math.max(16, Math.round((w * canvas.height) / canvas.width));
  const small = document.createElement("canvas");
  small.width = w;
  small.height = h;
  const ctx = small.getContext("2d", { willReadFrequently: true });
  if (!ctx) return Number.POSITIVE_INFINITY;
  ctx.drawImage(canvas, 0, 0, w, h);
  const { data } = ctx.getImageData(0, 0, w, h);
  const g = (x: number, y: number) => {
    const i = (y * w + x) * 4;
    return 0.2126 * data[i]! + 0.7152 * data[i + 1]! + 0.0722 * data[i + 2]!;
  };
  let sum = 0;
  let sq = 0;
  let n = 0;
  for (let y = 1; y < h - 1; y++) {
    for (let x = 1; x < w - 1; x++) {
      const l = g(x, y - 1) + g(x, y + 1) + g(x - 1, y) + g(x + 1, y) - 4 * g(x, y);
      sum += l;
      sq += l * l;
      n++;
    }
  }
  const mean = sum / n;
  return sq / n - mean * mean;
}
