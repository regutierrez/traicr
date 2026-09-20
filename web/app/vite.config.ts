import tailwindcss from '@tailwindcss/vite';
import { sveltekit } from '@sveltejs/kit/vite';
import { defineConfig } from 'vitest/config';

const server = 'http://127.0.0.1:8080';

type Request = { url?: string; method?: string };

// Some browser paths are Svelte pages for GET and Go handlers otherwise, or Go handlers only for
// particular sub-paths. Returning the URL from bypass keeps the request in Vite.
function pageUnless(isServerRequest: (request: Request) => boolean) {
	return {
		target: server,
		bypass: (request: Request) => (isServerRequest(request) ? undefined : request.url)
	};
}

const isMutation = (request: Request) => request.method !== 'GET' && request.method !== 'HEAD';
const pathOf = (request: Request) => new URL(request.url ?? '/', server);

export default defineConfig({
	plugins: [tailwindcss(), sveltekit()],
	test: {
		include: ['src/**/*.test.ts']
	},
	server: {
		host: '127.0.0.1',
		port: 5173,
		strictPort: true,
		proxy: {
			'/api': server,
			// Keep the browser Host so Go's Origin check matches Vite, same as /login.
			'/logout': { target: server },
			'/static': server,
			'/login': pageUnless(isMutation),
			'/traces': pageUnless((request) => isMutation(request) || /\/(events|transcript|resolve)$/.test(pathOf(request).pathname)),
			'/revisions': pageUnless((request) => {
				const url = pathOf(request);
				return /\/(attachment|block)$/.test(url.pathname) || url.searchParams.get('download') === '1';
			})
		}
	}
});
