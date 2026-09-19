import { redirect } from '@sveltejs/kit';
import { readJSON } from '$lib/api';
import { loadTranscriptEvents } from '$lib/transcript-load';
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
	if (!result.ok) {
		return { trace: null, revision: '', session: null, eventCount: 0, sessionError: '', reload: false, error: result.message };
	}
	const revision = url.searchParams.get('revision') ?? '';
	const known = result.data.revisions?.some((item) => String(item.id) === revision);
	if (revision && (result.data.harness !== 'amp' || !known)) {
		return {
			trace: null,
			revision,
			session: null,
			eventCount: 0,
			sessionError: '',
			reload: false,
			error: 'Revision not found for this Amp trace'
		};
	}
	const transcript = await loadTranscriptEvents(fetch, {
		traceId: String(result.data.id),
		nativeId: result.data.native_trace_id,
		harness: result.data.harness,
		title: result.data.title,
		cwd: result.data.working_directory,
		revision
	});
	if (!transcript.ok) {
		return {
			trace: result.data,
			revision,
			session: null,
			eventCount: 0,
			sessionError: transcript.message,
			reload: transcript.reload,
			error: ''
		};
	}
	return {
		trace: result.data,
		revision,
		session: transcript.session,
		eventCount: transcript.eventCount,
		sessionError: '',
		reload: false,
		error: ''
	};
};
