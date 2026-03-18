import { ConnectError } from '@connectrpc/connect';

export function getErrorMessage(err: unknown, fallback = 'An unexpected error occurred'): string {
	if (err instanceof ConnectError) return err.message;
	if (err instanceof Error) return err.message;
	return fallback;
}
