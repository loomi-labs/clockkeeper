import { writable } from "svelte/store";

const STORAGE_KEY = "clockkeeper_sidebar";

function getStored(): boolean {
  if (typeof localStorage === "undefined") return true;
  const stored = localStorage.getItem(STORAGE_KEY);
  if (stored === "false") return false;
  return true;
}

export const sidebarExpanded = writable<boolean>(getStored());

export function toggleSidebar() {
  sidebarExpanded.update((v) => {
    const next = !v;
    localStorage.setItem(STORAGE_KEY, String(next));
    return next;
  });
}

export function initSidebar() {
  sidebarExpanded.set(getStored());
}
