import { writable } from 'svelte/store';
import { goto } from '$app/navigation';

const TOKEN_KEY = 'clockkeeper_token';

export const isAuthenticated = writable(false);

export function getToken(): string | null {
	return localStorage.getItem(TOKEN_KEY);
}

export function setToken(token: string) {
	localStorage.setItem(TOKEN_KEY, token);
	isAuthenticated.set(true);
}

export function clearToken() {
	localStorage.removeItem(TOKEN_KEY);
	isAuthenticated.set(false);
}

export function initAuth() {
	isAuthenticated.set(!!getToken());
}

export function logout() {
	clearToken();
	goto('/login');
}
