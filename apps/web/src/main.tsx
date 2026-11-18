import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import App from "./App";
import Kiosk from "./screens/kiosk/Kiosk";
import VerifyDtr from "./screens/VerifyDtr";
import "./styles.css";

// /kiosk is the office kiosk: a registered device, not a person's sign-in.
const isKiosk = window.location.pathname.replace(/\/+$/, "") === "/kiosk";

const verify = window.location.pathname.match(/^\/verify\/([a-zA-Z0-9]+)\/?$/);

createRoot(document.getElementById("root")!).render(
  <StrictMode>{verify ? <VerifyDtr code={verify[1]!} /> : isKiosk ? <Kiosk /> : <App />}</StrictMode>,
);
