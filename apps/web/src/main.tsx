import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import App from "./App";
import Kiosk from "./screens/kiosk/Kiosk";
import "./styles.css";

// /kiosk is the office kiosk: a registered device, not a person's sign-in.
const isKiosk = window.location.pathname.replace(/\/+$/, "") === "/kiosk";

createRoot(document.getElementById("root")!).render(
  <StrictMode>{isKiosk ? <Kiosk /> : <App />}</StrictMode>,
);
