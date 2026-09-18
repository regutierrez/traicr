import tailwindcss from '@tailwindcss/vite';
import { sveltekit } from '@sveltejs/kit/vite';
import { defineConfig } from 'vite';

export default defineConfig({
	plugins: [tailwindcss(), sveltekit()],
	server: {
		host: '127.0.0.1',
		port: 5173,
		strictPort: true,
		proxy: {
			'/api': 'http://127.0.0.1:8080',
			'/logout': 'http://127.0.0.1:8080',
			'/static': 'http://127.0.0.1:8080',
			'/login': {
				target: 'http://127.0.0.1:8080',
				bypass: (req) => (req.method === 'GET' ? req.url : undefined)
			},
			'/traces': {
				target: 'http://127.0.0.1:8080',
				bypass: (req) => {
					const url = req.url ?? '';
					if (url.includes('/events') || url.includes('/transcript') || url.includes('/resolve')) return undefined;
					if (req.method !== 'GET' && req.method !== 'HEAD') return undefined;
					return url;
				}
			}
		}
	}
});
