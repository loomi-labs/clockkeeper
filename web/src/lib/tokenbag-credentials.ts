// Per-device token bag credentials.
//
// A player device stores the registration secret it got from JoinTokenBag: it is
// the only proof that this browser owns that registration, and it is handed back
// on every watch/neighbor/my-token call. Credentials are scoped per join code so
// one device can hold registrations for several games (and so a reset game with
// a fresh code never reuses a dead secret).
//
// Everything here is defensive: a corrupt or unavailable localStorage must never
// throw into the join flow — the worst outcome is the player registering again.

export type TokenBagCredential = {
  /** Proto int64 arrives as bigint; persisted (and compared) as a string. */
  registrationId: string;
  secret: string;
  name: string;
};

const CREDENTIAL_PREFIX = "clockkeeper_player:";
const DISMISSED_PREFIX = "clockkeeper_player_dismissed:";

function credentialKey(code: string): string {
  return `${CREDENTIAL_PREFIX}${code}`;
}

function dismissedKey(code: string): string {
  return `${DISMISSED_PREFIX}${code}`;
}

/** localStorage, or null when it is missing (SSR) or blocked (private mode). */
function storage(): Storage | null {
  try {
    return typeof localStorage === "undefined" ? null : localStorage;
  } catch {
    return null;
  }
}

function read(key: string): string | null {
  try {
    return storage()?.getItem(key) ?? null;
  } catch {
    return null;
  }
}

function write(key: string, value: string) {
  try {
    storage()?.setItem(key, value);
  } catch {
    // Full or blocked storage: the credential is lost, not fatal.
  }
}

function remove(key: string) {
  try {
    storage()?.removeItem(key);
  } catch {
    // Nothing to do — see write().
  }
}

/** Returns null for a missing, unparsable or incomplete credential. */
export function loadCredential(code: string): TokenBagCredential | null {
  const raw = read(credentialKey(code));
  if (!raw) return null;
  let parsed: unknown;
  try {
    parsed = JSON.parse(raw);
  } catch {
    return null;
  }
  if (typeof parsed !== "object" || parsed === null) return null;
  const { registrationId, secret, name } = parsed as Record<string, unknown>;
  if (typeof secret !== "string" || secret === "") return null;
  if (
    typeof registrationId !== "string" &&
    typeof registrationId !== "number"
  ) {
    return null;
  }
  return {
    registrationId: String(registrationId),
    secret,
    name: typeof name === "string" ? name : "",
  };
}

export function saveCredential(
  code: string,
  cred: { registrationId: string | bigint; secret: string; name: string },
) {
  const value: TokenBagCredential = {
    registrationId: String(cred.registrationId),
    secret: cred.secret,
    name: cred.name,
  };
  write(credentialKey(code), JSON.stringify(value));
}

/**
 * Forgets this device's registration. The dismissed flag goes with it — it is a
 * property of the credential ("I already looked at my token"), so leaving it
 * behind would hide the token of whoever registers next on this device.
 */
export function clearCredential(code: string) {
  remove(credentialKey(code));
  remove(dismissedKey(code));
}

/** True once the player has hidden their revealed token on this device. */
export function isDismissed(code: string): boolean {
  return read(dismissedKey(code)) === "true";
}

export function setDismissed(code: string, dismissed: boolean) {
  if (dismissed) {
    write(dismissedKey(code), "true");
  } else {
    remove(dismissedKey(code));
  }
}
