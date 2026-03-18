import { sveltekit } from '@sveltejs/kit/vite';
import tailwindcss from '@tailwindcss/vite';
import { defineConfig, loadEnv } from 'vite';

export default defineConfig(({ mode }) => {
	const env = loadEnv(mode, '..', '');
	const listen = env.WEB_LISTEN || ':8080';
	const port = listen.split(':').pop();
	const target = `http://localhost:${port}`;

	return {
		plugins: [tailwindcss(), sveltekit()],
		server: {
			proxy: {
				'/clockkeeper.v1.ClockKeeperService': { target, changeOrigin: true },
				'/characters': { target, changeOrigin: true }
			}
		}
	};
});
