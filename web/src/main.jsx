import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import { BrowserRouter } from "react-router-dom";
import { App } from "./App.jsx";
import "./styles.css";

const touch = window.matchMedia("(pointer: coarse)");
document.addEventListener("contextmenu", (event) => {
  if (!touch.matches) return;
  if (event.target.closest("input, textarea, [contenteditable='true']")) return;
  event.preventDefault();
});

createRoot(document.getElementById("root")).render(
  <StrictMode>
    <BrowserRouter>
      <App />
    </BrowserRouter>
  </StrictMode>,
);
