import { redirect } from '@sveltejs/kit';
import { readJSON } from '$lib/api';
import { loadTranscriptSession } from '$lib/transcript/load';
import { latestRevisionId } from '$lib/transcript/revision';
import type { LoadedSession } from '$lib/transcript/types';
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
	if (!result.ok) return { trace: null, revision: '', session: null, loadError: '', error: result.message };
	if (url.searchParams.has('revision')) {
		url.searchParams.delete('revision');
		const query = url.searchParams.toString();
		redirect(303, `/traces/${params.id}${query ? `?${query}` : ''}`);
	}
	const revision = result.data.harness === 'amp' ? latestRevisionId(result.data.revisions) : '';
	try {
		const session = await loadTranscriptSession(
			{
				traceId: params.id,
				nativeId: result.data.native_trace_id,
				harness: result.data.harness,
				title: result.data.title,
				cwd: result.data.working_directory,
				revision
			},
			fetch
		);
		return { trace: result.data, revision, session, loadError: '', error: '' };
	} catch (error) {
		return {
			trace: result.data,
			revision,
			session: null as LoadedSession | null,
			loadError: error instanceof Error ? error.message : 'Transcript load failed. Reload the page and sign in again.',
			error: ''
		};
	}
};
