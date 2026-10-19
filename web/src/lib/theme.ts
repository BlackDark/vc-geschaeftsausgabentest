export type Theme = "system" | "light" | "dark";

const key = "spesen.theme";

function readStored(): Theme {
  try {
    const v = localStorage.getItem(key);
    if (v === "light" || v === "dark" || v === "system") return v;
  } catch {
    // storage unavailable: fall back to system
  }
  return "system";
}

/** Applies the theme to <html> and remembers it for the next visit. */
export function applyTheme(theme: Theme) {
  try {
    localStorage.setItem(key, theme);
  } catch {
    // ignore
  }
  const dark =
    theme === "dark" || (theme === "system" && window.matchMedia("(prefers-color-scheme: dark)").matches);
  document.documentElement.classList.toggle("dark", dark);
  document.querySelector('meta[name="theme-color"]')?.setAttribute("content", dark ? "#1b2033" : "#1e40af");
}

export function applyStoredTheme() {
  applyTheme(readStored());
  window
    .matchMedia("(prefers-color-scheme: dark)")
    .addEventListener("change", () => applyTheme(readStored()));
}
