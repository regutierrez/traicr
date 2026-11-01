import { error, redirect } from '@sveltejs/kit';

type Failure = { error?: { message?: string } };

export async function readJSON<T>(
	fetch: typeof globalThis.fetch,
	path: string
): Promise<{ ok: true; data: T } | { ok: false; message: string }> {
	let response: Response;
	try {
		response = await fetch(path, { headers: { accept: 'application/json' } });
	} catch {
		// fetch rejects only when no answer came back at all; +error.svelte shows this with a Reload button.
		error(503, 'The Traicr server did not answer. Check that it is running, then reload.');
	}
	// Come back here after login. During a client navigation the address bar still shows the page they came from.
	if (response.status === 401) redirect(303, `/login?next=${encodeURIComponent(location.pathname + location.search)}`);
	const body = (await response.json().catch(() => ({}))) as T & Failure;
	if (!response.ok) return { ok: false, message: body.error?.message ?? 'request failed' };
	return { ok: true, data: body };
}
