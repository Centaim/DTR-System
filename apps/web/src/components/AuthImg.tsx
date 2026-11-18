import { useEffect, useState } from "react";
import { api } from "../lib/api";
import { useT } from "../lib/i18n";

/** Loads a protected photo with the session token. Every load is logged server-side. */
export function AuthImg({ logId, alt }: { logId: number; alt: string }) {
  const { t } = useT();
  const [state, setState] = useState<{ url: string } | "loading" | "missing">("loading");

  useEffect(() => {
    let live = true;
    let url = "";
    setState("loading");
    api.audit
      .photo(logId)
      .then((blob) => {
        if (!live) return;
        url = URL.createObjectURL(blob);
        setState({ url });
      })
      .catch(() => live && setState("missing"));
    return () => {
      live = false;
      if (url) URL.revokeObjectURL(url);
    };
  }, [logId]);

  if (state === "loading") return <p className="photo-note muted">{t.photoLoading}</p>;
  if (state === "missing") return <p className="photo-note muted">{t.photoMissing}</p>;
  return <img className="audit-photo" src={state.url} alt={alt} />;
}
