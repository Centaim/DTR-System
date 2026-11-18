import { useCallback, useEffect, useState, type FormEvent } from "react";
import { LocateFixed } from "lucide-react";
import { ApiError, api, type DeviceRequest, type Geofence } from "../../lib/api";
import { useT } from "../../lib/i18n";

const blank = { name: "", latitude: "", longitude: "", radius_m: "150", office: "" };

export function LocationsTab() {
  const { t } = useT();
  const [fences, setFences] = useState<Geofence[] | null>(null);
  const [devices, setDevices] = useState<DeviceRequest[]>([]);
  const [failed, setFailed] = useState(false);
  const [f, setF] = useState(blank);
  const [error, setError] = useState("");

  const load = useCallback(() => {
    setFailed(false);
    Promise.all([api.admin.geofences(), api.admin.deviceRequests()])
      .then(([g, d]) => {
        setFences(g);
        setDevices(d);
      })
      .catch(() => setFailed(true));
  }, []);
  useEffect(load, [load]);

  const useHere = () => {
    setError("");
    navigator.geolocation?.getCurrentPosition(
      (p) => setF((x) => ({ ...x, latitude: p.coords.latitude.toFixed(6), longitude: p.coords.longitude.toFixed(6) })),
      () => setError(t.geoLocFailed),
      { enableHighAccuracy: true, timeout: 8000 },
    );
  };

  const add = async (ev: FormEvent) => {
    ev.preventDefault();
    setError("");
    try {
      await api.admin.createGeofence({
        name: f.name,
        latitude: Number(f.latitude),
        longitude: Number(f.longitude),
        radius_m: Number(f.radius_m),
        office: f.office,
      });
      setF(blank);
      load();
    } catch (e) {
      setError(e instanceof ApiError && e.message ? e.message : t.errGeneric);
    }
  };

  return (
    <div className="admin-cols">
      <div className="stack">
        <section className="panel" aria-labelledby="geo-h">
          <h2 id="geo-h" className="panel-h">{t.geoTitle}</h2>
          <p className="muted small">{t.geoNote}</p>
          {failed && (
            <p className="msg bad" role="alert">
              {t.loadFailed} <button className="link-btn" onClick={load}>{t.retry}</button>
            </p>
          )}
          {fences?.length === 0 && <p className="muted">{t.geoEmpty}</p>}
          <ul className="rows flat">
            {fences?.map((g) => (
              <li key={g.id} className="row">
                <span className="row-main">
                  <span className="rail-slot">{g.name}</span>
                  <span className="rail-state">
                    {g.latitude.toFixed(5)}, {g.longitude.toFixed(5)} · {g.radius_m} m{g.office ? ` · ${g.office}` : ""}
                  </span>
                </span>
                <button className="link-btn" onClick={() => api.admin.deleteGeofence(g.id).finally(load)}>{t.geoRemove}</button>
              </li>
            ))}
          </ul>
        </section>

        <section className="panel" aria-labelledby="dev-h">
          <h2 id="dev-h" className="panel-h">{t.devTitle}</h2>
          <p className="muted small">{t.devNote}</p>
          {devices.length === 0 && <p className="muted">{t.devEmpty}</p>}
          <ul className="rows flat">
            {devices.map((d) => (
              <li key={d.id} className="row approval">
                <span className="row-main">
                  <span className="rail-slot">{d.name}</span>
                  <span className="rail-state">{d.employee_no} · {d.device_id.slice(0, 8)}</span>
                  <span className="actions">
                    <button className="btn primary" onClick={() => api.admin.decideDevice(d.id, true).finally(load)}>{t.devApprove}</button>
                    <button className="btn quiet" onClick={() => api.admin.decideDevice(d.id, false).finally(load)}>{t.devReject}</button>
                  </span>
                </span>
              </li>
            ))}
          </ul>
        </section>
      </div>

      <form className="panel form" onSubmit={add} noValidate>
        <h2 className="panel-h">{t.geoAdd}</h2>
        <label className="field">
          <span>{t.geoName}</span>
          <input value={f.name} maxLength={80} onChange={(e) => setF({ ...f, name: e.target.value })} />
        </label>
        <button type="button" className="btn quiet" onClick={useHere}>
          <LocateFixed aria-hidden="true" size={20} /> {t.geoUseHere}
        </button>
        <div className="two">
          <label className="field">
            <span>{t.geoLat}</span>
            <input inputMode="decimal" value={f.latitude} onChange={(e) => setF({ ...f, latitude: e.target.value })} />
          </label>
          <label className="field">
            <span>{t.geoLng}</span>
            <input inputMode="decimal" value={f.longitude} onChange={(e) => setF({ ...f, longitude: e.target.value })} />
          </label>
        </div>
        <label className="field">
          <span>{t.geoRadius}</span>
          <input type="number" min={20} max={20000} value={f.radius_m} onChange={(e) => setF({ ...f, radius_m: e.target.value })} />
        </label>
        <label className="field">
          <span>{t.geoOffice}</span>
          <input value={f.office} maxLength={80} onChange={(e) => setF({ ...f, office: e.target.value })} />
        </label>
        <div aria-live="polite">{error && <p className="msg bad">{error}</p>}</div>
        <button className="btn primary" type="submit" disabled={!f.name.trim() || !f.latitude || !f.longitude}>{t.geoAdd}</button>
      </form>
    </div>
  );
}
