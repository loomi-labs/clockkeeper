import { createConnectTransport } from "@connectrpc/connect-web";
import {
  ConnectError,
  Code,
  createClient,
  type Interceptor,
} from "@connectrpc/connect";
import { ClockKeeperService } from "./gen/clockkeeper/v1/clockkeeper_pb";
import { auth, getToken, setToken, setAnonymous, logout } from "./auth.svelte";

const authInterceptor: Interceptor = (next) => async (req) => {
  const token = getToken();
  if (token) {
    req.header.set("Authorization", `Bearer ${token}`);
  }
  try {
    return await next(req);
  } catch (err) {
    if (err instanceof ConnectError) {
      if (err.code === Code.Unauthenticated) {
        // For anonymous users, try to create a new session before logging out.
        if (auth.isAnonymous) {
          try {
            const resp = await rawClient.createAnonymousSession({});
            setToken(resp.token);
            setAnonymous(true);
            // Retry the original request isn't practical here,
            // so just reload to restart with the new token.
            window.location.reload();
            // Throw to prevent further execution.
            throw err;
          } catch {
            // Fall through to logout.
          }
        }
        logout();
      }
      if (err.code === Code.ResourceExhausted) {
        throw new ConnectError(
          "Too many requests. Please wait a moment and try again.",
          Code.ResourceExhausted,
        );
      }
    }
    throw err;
  }
};

const transport = createConnectTransport({
  baseUrl: window.location.origin,
  interceptors: [authInterceptor],
});

// Raw client without interceptors for bootstrap calls (createAnonymousSession).
const rawTransport = createConnectTransport({
  baseUrl: window.location.origin,
});

const rawClient = createClient(ClockKeeperService, rawTransport);

export const client = createClient(ClockKeeperService, transport);
export { rawClient };
