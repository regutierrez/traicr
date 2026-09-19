import { readJSON } from '$lib/api';
import type { LayoutLoad } from './$types';

export const ssr = false;
export const prerender = false;

// Every form posted to the Go server carries this token; the endpoint mints a login cookie when there is none.
export const load: LayoutLoad = async ({ fetch }) => {
	const result = await readJSON<{ csrf?: string }>(fetch, '/api/v1/csrf');
	return { csrf: result.ok ? (result.data.csrf ?? '') : '' };
};
