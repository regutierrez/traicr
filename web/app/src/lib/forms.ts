type Failure = { error?: { message?: string } };

// Go mutation handlers return JSON on failure and 303 + HTML on success.
// Native form navigation would replace the Svelte document with that JSON.
//
// The CSRF token is refreshed with window.fetch immediately before POST so it
// matches the cookie the browser will actually send. SvelteKit's load fetch can
// mint a token for a cookie that never lands in the jar.
export async function submitGoForm(form: HTMLFormElement): Promise<{ ok: true; url: string } | { ok: false; message: string }> {
	const body = new URLSearchParams();
	for (const [name, value] of new FormData(form)) {
		if (typeof value === 'string') body.set(name, value);
	}

	const csrf = await fetch('/api/v1/csrf', {
		credentials: 'same-origin',
		headers: { accept: 'application/json' }
	});
	const token = (await csrf.json().catch(() => ({}))) as { csrf?: string };
	if (typeof token.csrf === 'string' && token.csrf !== '') {
		body.set('csrf', token.csrf);
	}

	const response = await fetch(form.action, {
		method: 'post',
		body,
		credentials: 'same-origin',
		headers: { accept: 'application/json' }
	});
	// fetch follows the 303; redirected is the success signal. An unredirected
	// HTML body is not success — that used to hide JSON errors rendered as a document.
	if (response.redirected) {
		return { ok: true, url: response.url || '/' };
	}

	const type = response.headers.get('content-type') ?? '';
	if (type.includes('application/json')) {
		const payload = (await response.json().catch(() => ({}))) as Failure;
		return { ok: false, message: payload.error?.message ?? 'request failed' };
	}
	return { ok: false, message: 'request failed' };
}
