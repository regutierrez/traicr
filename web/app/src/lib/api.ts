import { error, redirect } from '@sveltejs/kit';

type Failure = { error?: { message?: string } };

export async function readJSON<T>(
	fetch: typeof globalThis.fetch,
	path: string,
	/** The page this load opens: the `url` a load function gets. It is the link target even during a hover preload. */
	opening?: URL
): Promise<{ ok: true; data: T } | { ok: false; status: number; message: string }> {
	let response: Response;
	try {
		response = await fetch(path, { headers: { accept: 'application/json' } });
	} catch {
		// fetch rejects only when no answer came back at all; +error.svelte shows this with a Reload button.
		error(503, 'The Traicr server did not answer. Check that it is running, then reload.');
	}
	// Come back here after login: to the page the load opens, else the current one.
	if (response.status === 401) {
		const target = opening ?? location;
		redirect(303, `/login?next=${encodeURIComponent(target.pathname + target.search)}`);
	}
	const body = (await response.json().catch(() => ({}))) as T & Failure;
	if (!response.ok) return { ok: false, status: response.status, message: body.error?.message ?? 'request failed' };
	return { ok: true, data: body };
}

/** Shows +error.svelte with `message` when the record is missing: 404, or 400 for an ID that is not a number. */
export function stopIfMissing(result: { ok: boolean; status?: number }, message: string) {
	if (!result.ok && (result.status === 404 || result.status === 400)) error(404, message);
}
