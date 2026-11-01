import { readJSON, stopIfMissing } from '$lib/api';
import { loadTranscriptSession, reportLoad } from '$lib/transcript/load';
import type { LoadedSession } from '$lib/transcript/types';
import type { Trace } from '$lib/types';
import type { LayoutLoad } from './$types';

// Overview and Trace share one load, so switching tabs does not fetch the transcript again.
export const load: LayoutLoad = async ({ fetch, params, url }) => {
	const result = await readJSON<Trace>(fetch, `/api/v1/traces/${params.id}`, url);
	stopIfMissing(result, 'This session is not in the archive. It may have been deleted.');
	if (!result.ok) return { trace: null, revision: '', session: null, loadError: '', error: result.message };
	const revision = url.searchParams.get('revision') ?? '';
	const known = result.data.revisions?.some((item) => String(item.id) === revision);
	if (revision && (result.data.harness !== 'amp' || !known)) {
		return { trace: null, revision, session: null, loadError: '', error: 'Revision not found for this Amp trace' };
	}
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
		void reportLoad(session);
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
