import { readJSON } from '$lib/api';
import type { SessionNative } from '$lib/transcript/types';
import type { EventPage, Trace } from '$lib/types';
import type { PageLoad } from './$types';

export const load: PageLoad = async ({ fetch, params, url, parent }) => {
	const { csrf } = await parent();
	const cursor = url.searchParams.get('cursor') ?? '';
	const key = url.searchParams.get('key') ?? '';
	const query = new URLSearchParams();
	if (cursor) query.set('cursor', cursor);
	if (key) query.set('key', key);
	const [trace, events] = await Promise.all([
		readJSON<Trace>(fetch, `/api/v1/traces/${params.id}`),
		readJSON<EventPage>(fetch, `/api/v1/traces/${params.id}/events?${query.toString()}`)
	]);
	if (!trace.ok) return { session: null, trace: null, events: [], next: '', error: trace.message, csrf };
	if (!events.ok) return { session: null, trace: trace.data, events: [], next: '', error: events.message, csrf };
	// An Amp export records the session it came from: agent mode, repository, client.
	let session: SessionNative | null = null;
	if (trace.data.harness === 'amp') {
		const info = await readJSON<EventPage>(fetch, `/api/v1/traces/${params.id}/events?key=${encodeURIComponent(`session_info:${trace.data.native_trace_id}`)}&limit=1`);
		const first = info.ok ? info.data.events?.[0] : undefined;
		if (first?.kind === 'session_info') session = (first.metadata as { session?: SessionNative } | undefined)?.session ?? null;
	}
	return {
		session,
		trace: trace.data,
		events: events.data.events ?? [],
		next: events.data.next_cursor ?? '',
		error: '',
		csrf
	};
};
