import { readJSON } from '$lib/api';
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
	if (!trace.ok) return { trace: null, events: [], next: '', error: trace.message, csrf };
	if (!events.ok) return { trace: trace.data, events: [], next: '', error: events.message, csrf };
	return {
		trace: trace.data,
		events: events.data.events ?? [],
		next: events.data.next_cursor ?? '',
		error: '',
		csrf
	};
};
