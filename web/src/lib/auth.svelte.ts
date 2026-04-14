import { goto } from "$app/navigation";

const TOKEN_KEY = "clockkeeper_token";
const ANON_KEY = "clockkeeper_anonymous";

export const auth = $state({
  isAuthenticated: false,
  isAnonymous: false,
  discordAvailable: false,
  discordClientId: "",
});

export function getToken(): string | null {
  return localStorage.getItem(TOKEN_KEY);
}

export function setToken(token: string) {
  localStorage.setItem(TOKEN_KEY, token);
  auth.isAuthenticated = true;
}

export function clearToken() {
  localStorage.removeItem(TOKEN_KEY);
  localStorage.removeItem(ANON_KEY);
  auth.isAuthenticated = false;
  auth.isAnonymous = false;
}

export function setAnonymous(value: boolean) {
  if (value) {
    localStorage.setItem(ANON_KEY, "true");
  } else {
    localStorage.removeItem(ANON_KEY);
  }
  auth.isAnonymous = value;
}

export function initAuth() {
  auth.isAuthenticated = !!getToken();
  auth.isAnonymous = localStorage.getItem(ANON_KEY) === "true";
}

const OAUTH_STATE_KEY = "discord_oauth_state";

function generateOAuthState(): string {
  const bytes = crypto.getRandomValues(new Uint8Array(32));
  return Array.from(bytes, (b) => b.toString(16).padStart(2, "0")).join("");
}

export function getDiscordOAuthURL(): string {
  const redirectUri = `${window.location.origin}/auth/discord/callback`;
  const state = generateOAuthState();
  sessionStorage.setItem(OAUTH_STATE_KEY, state);
  const params = new URLSearchParams({
    client_id: auth.discordClientId,
    redirect_uri: redirectUri,
    response_type: "code",
    scope: "identify",
    state,
  });
  return `https://discord.com/oauth2/authorize?${params}`;
}

export function validateOAuthState(state: string | null): boolean {
  const expected = sessionStorage.getItem(OAUTH_STATE_KEY);
  sessionStorage.removeItem(OAUTH_STATE_KEY);
  return !!state && !!expected && state === expected;
}

export function logout() {
  clearToken();
  goto("/login");
}
