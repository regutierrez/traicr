import { readJSON } from '$lib/api';
import type { LayoutLoad } from './$types';

export const ssr = false;
export const prerender = false;

// Every form posted to the Go server carries this token; the endpoint mints a login cookie when there is none.
export const load: LayoutLoad = async ({ fetch }) => {
	// An error here would skip +error.svelte (it renders inside this layout), so the page's own load reports an unreachable server.
	const result = await readJSON<{ csrf?: string }>(fetch, '/api/v1/csrf').catch(() => null);
	return { csrf: result?.ok ? (result.data.csrf ?? '') : '' };
};
