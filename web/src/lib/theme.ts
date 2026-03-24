import { writable, derived } from "svelte/store";

export type Theme = "light" | "dark" | "tavern" | "crypt";

const STORAGE_KEY = "clockkeeper_theme";
const VALID_THEMES: Theme[] = ["light", "dark", "tavern", "crypt"];

function isDarkTheme(t: Theme): boolean {
  return t === "dark" || t === "crypt";
}

function getStored(): Theme {
  if (typeof localStorage === "undefined") return "light";
  const stored = localStorage.getItem(STORAGE_KEY);
  if (VALID_THEMES.includes(stored as Theme)) return stored as Theme;
  if (
    typeof window !== "undefined" &&
    window.matchMedia("(prefers-color-scheme: dark)").matches
  ) {
    return "dark";
  }
  return "light";
}

function applyTheme(t: Theme) {
  const html = document.documentElement;
  html.setAttribute("data-theme", t);
  html.classList.toggle("dark", isDarkTheme(t));
}

export const theme = writable<Theme>(getStored());

export const isDark = derived(theme, ($theme) => isDarkTheme($theme));

// --- Preview (instant, no animation, no save) ---
export function applyPreview(next: Theme) {
  applyTheme(next);
  theme.set(next);
}

export function revertPreview() {
  const saved = getStored();
  applyTheme(saved);
  theme.set(saved);
}

// --- Set theme (commit with circular animation + save) ---
export function setTheme(next: Theme, event?: MouseEvent) {
  // Revert to saved state first so animation starts from the correct theme
  const saved = getStored();
  applyTheme(saved);

  function apply() {
    theme.set(next);
    localStorage.setItem(STORAGE_KEY, next);
    applyTheme(next);
  }

  if (!event || !document.startViewTransition) {
    apply();
    return;
  }

  const el = event.currentTarget as HTMLElement;
  const rect = el.getBoundingClientRect();
  const x = rect.left + rect.width / 2;
  const y = rect.top + rect.height / 2;
  const endRadius = Math.hypot(
    Math.max(x, innerWidth - x),
    Math.max(y, innerHeight - y),
  );

  document.documentElement.style.setProperty("--vt-x", x + "px");
  document.documentElement.style.setProperty("--vt-y", y + "px");
  document.documentElement.style.setProperty("--vt-r", endRadius + "px");

  const goingDark = isDarkTheme(next);
  const t = document.startViewTransition(apply);
  t.ready.then(() => {
    const full = "circle(var(--vt-r) at var(--vt-x) var(--vt-y))";
    const zero = "circle(0px at var(--vt-x) var(--vt-y))";
    const opts = { duration: 400, easing: "ease-in-out" };
    if (goingDark) {
      document.documentElement.animate(
        { clipPath: [full, zero] },
        { ...opts, pseudoElement: "::view-transition-old(root)" },
      );
      document.documentElement.animate(
        { zIndex: [2, 2] },
        { duration: 400, pseudoElement: "::view-transition-old(root)" },
      );
      document.documentElement.animate(
        { zIndex: [1, 1] },
        { duration: 400, pseudoElement: "::view-transition-new(root)" },
      );
    } else {
      document.documentElement.animate(
        { clipPath: [zero, full] },
        { ...opts, pseudoElement: "::view-transition-new(root)" },
      );
    }
  });
}

export function initTheme() {
  const stored = getStored();
  theme.set(stored);
  applyTheme(stored);
}
