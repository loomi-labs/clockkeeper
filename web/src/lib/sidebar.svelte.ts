const STORAGE_KEY = 'clockkeeper_sidebar';

function getStored(): boolean {
	if (typeof localStorage === 'undefined') return true;
	const stored = localStorage.getItem(STORAGE_KEY);
	if (stored === 'false') return false;
	return true;
}

export const sidebar = $state({ expanded: getStored() });

export function toggleSidebar() {
	sidebar.expanded = !sidebar.expanded;
	localStorage.setItem(STORAGE_KEY, String(sidebar.expanded));
}

export function initSidebar() {
	sidebar.expanded = getStored();
}
