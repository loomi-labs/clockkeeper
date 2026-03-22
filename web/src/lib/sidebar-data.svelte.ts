export const sidebarData = $state({ version: 0 });

export function invalidateSidebar() {
	sidebarData.version++;
}
