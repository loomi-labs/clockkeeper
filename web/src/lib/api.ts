import { createConnectTransport } from '@connectrpc/connect-web';
import { ConnectError, Code, createClient, type Interceptor } from '@connectrpc/connect';
import { ClockKeeperService } from './gen/clockkeeper/v1/clockkeeper_pb';
import { getToken, logout } from './auth.svelte';

const authInterceptor: Interceptor = (next) => async (req) => {
	const token = getToken();
	if (token) {
		req.header.set('Authorization', `Bearer ${token}`);
	}
	try {
		return await next(req);
	} catch (err) {
		if (err instanceof ConnectError && err.code === Code.Unauthenticated) {
			logout();
		}
		throw err;
	}
};

const transport = createConnectTransport({
	baseUrl: window.location.origin,
	interceptors: [authInterceptor]
});

export const client = createClient(ClockKeeperService, transport);
