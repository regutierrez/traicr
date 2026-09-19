import { redirect } from '@sveltejs/kit';
import { readJSON } from '$lib/api';
import type { Trace } from '$lib/types';
import type { PageLoad } from './$types';

export const load: PageLoad = async ({ fetch, params, url }) => {
	if (url.searchParams.get('view') === 'records') {
		const next = new URLSearchParams(url.searchParams);
		next.delete('view');
		const query = next.toString();
		redirect(303, `/traces/${params.id}/records${query ? `?${query}` : ''}`);
	}
	const result = await readJSON<Trace>(fetch, `/api/v1/traces/${params.id}`);
	if (!result.ok) return { trace: null, revision: '', error: result.message };
	const revision = url.searchParams.get('revision') ?? '';
	const known = result.data.revisions?.some((item) => String(item.id) === revision);
	if (revision && (result.data.harness !== 'amp' || !known)) {
		return { trace: null, revision, error: 'Revision not found for this Amp trace' };
	}
	return { trace: result.data, revision, error: '' };
};
