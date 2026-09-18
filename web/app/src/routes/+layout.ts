import type { LayoutLoad } from './$types';

export const ssr = false;
export const prerender = false;

export const load: LayoutLoad = async ({ fetch }) => {
	const response = await fetch('/api/v1/csrf', { headers: { accept: 'application/json' } });
	if (!response.ok) return { csrf: '' };
	const body = (await response.json()) as { csrf?: string };
	return { csrf: body.csrf ?? '' };
};
