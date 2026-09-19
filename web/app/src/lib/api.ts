import { redirect } from '@sveltejs/kit';

type Failure = { error?: { message?: string } };

export async function readJSON<T>(
	fetch: typeof globalThis.fetch,
	path: string
): Promise<{ ok: true; data: T } | { ok: false; message: string }> {
	const response = await fetch(path, { headers: { accept: 'application/json' } });
	if (response.status === 401) redirect(303, '/login');
	const body = (await response.json().catch(() => ({}))) as T & Failure;
	if (!response.ok) return { ok: false, message: body.error?.message ?? 'request failed' };
	return { ok: true, data: body };
}
