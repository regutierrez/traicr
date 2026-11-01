import { redirect } from '@sveltejs/kit';
import type { PageLoad } from './$types';

// Links made before Overview existed point a trace URL at a message, branch, or
// filter, or ask for the old records view; those belong to the Trace tab or here.
const traceParams = ['event', 'key', 'leafId', 'targetId', 'filter', 'debug'];

export const load: PageLoad = ({ params, url }) => {
	const query = new URLSearchParams(url.searchParams);
	const old = query.get('view') === 'records';
	query.delete('view');
	if (traceParams.some((name) => query.has(name))) redirect(307, `/traces/${params.id}/trace?${query}`);
	if (old) redirect(307, `/traces/${params.id}${query.size ? `?${query}` : ''}`);
};
