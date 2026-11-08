import { readJSON, stopIfMissing } from '$lib/api';
import { loadTranscriptSession, reportLoad } from '$lib/transcript/load';
import type { LoadedSession } from '$lib/transcript/types';
import type { Trace } from '$lib/types';
import type { LayoutLoad } from './$types';

type Transcript = { session: LoadedSession | null; loadError: string };

// Overview, Trace, and Changes share one load, so switching tabs does not fetch the transcript again.
// The transcript is a promise that never rejects: Overview and Trace wait for it in their own loads,
// and Changes, which does not read it, opens without waiting for every transcript page.
export const load: LayoutLoad = async ({ fetch, params, url }) => {
	const none: Promise<Transcript> = Promise.resolve({ session: null, loadError: '' });
	const result = await readJSON<Trace>(fetch, `/api/v1/traces/${params.id}`, url);
	stopIfMissing(result, 'This session is not in the archive. It may have been deleted.');
	if (!result.ok) return { trace: null, revision: '', transcript: none, error: result.message };
	const revision = url.searchParams.get('revision') ?? '';
	const known = result.data.revisions?.some((item) => String(item.id) === revision);
	if (revision && (result.data.harness !== 'amp' || !known)) {
		return { trace: null, revision, transcript: none, error: 'Revision not found for this Amp trace' };
	}
	const transcript: Promise<Transcript> = loadTranscriptSession(
		{
			traceId: params.id,
			nativeId: result.data.native_trace_id,
			harness: result.data.harness,
			title: result.data.title,
			cwd: result.data.working_directory,
			revision
		},
		fetch
	).then(
		(session) => {
			void reportLoad(session);
			return { session, loadError: '' };
		},
		(error) => ({
			session: null,
			loadError: error instanceof Error ? error.message : 'Transcript load failed. Reload the page and sign in again.'
		})
	);
	return { trace: result.data, revision, transcript, error: '' };
};
