import { writable } from "svelte/store";

const sidebarVersion = writable(0);

export { sidebarVersion };

export function invalidateSidebar() {
  sidebarVersion.update((v) => v + 1);
}
