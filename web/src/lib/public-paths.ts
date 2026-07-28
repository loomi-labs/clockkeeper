// Which routes are reachable without a Storyteller session.
//
// Two things hang off this list in `+layout.svelte`: the render gate (public
// routes render bare, without the app shell) and the anonymous-session mint. A
// player scanning a QR code must NOT be handed a Storyteller session, so
// `/join/*` and `/device/*` have to be public in both places.

/**
 * Path prefixes that render without an authenticated session.
 *
 * A prefix ending in `/` only matches inside that segment, so `/joinery` is not
 * public. The bare entry points (`/join`, `/device`) are listed separately in
 * {@link PUBLIC_EXACT_PATHS}; `/login` keeps its historical plain-prefix
 * semantics (`/login-anything` is public) and `/auth` alone is not public.
 */
export const PUBLIC_PATH_PREFIXES = [
  "/login",
  "/auth/",
  "/join/",
  "/device/",
] as const;

/** Bare QR entry points — no code in the URL yet, still public. */
export const PUBLIC_EXACT_PATHS = ["/join", "/device"] as const;

export function isPublicPath(pathname: string): boolean {
  return (
    PUBLIC_EXACT_PATHS.some((path) => pathname === path) ||
    PUBLIC_PATH_PREFIXES.some((prefix) => pathname.startsWith(prefix))
  );
}
