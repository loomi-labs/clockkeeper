import { goto } from '$app/navigation';

const TOKEN_KEY = 'clockkeeper_token';

export const auth = $state({ isAuthenticated: false });

export function getToken(): string | null {
	return localStorage.getItem(TOKEN_KEY);
}

export function setToken(token: string) {
	localStorage.setItem(TOKEN_KEY, token);
	auth.isAuthenticated = true;
}

export function clearToken() {
	localStorage.removeItem(TOKEN_KEY);
	auth.isAuthenticated = false;
}

export function initAuth() {
	auth.isAuthenticated = !!getToken();
}

export function logout() {
	clearToken();
	goto('/login');
}
